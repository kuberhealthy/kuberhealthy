// Kuberhealthy is an enhanced health check for Kubernetes clusters.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/integrii/flaggy"
	log "github.com/sirupsen/logrus"

	zaplog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kuberhealthy/kuberhealthy/v3/internal/controller"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/kuberhealthy"
)

// GlobalConfig holds the configuration settings for Kuberhealthy
var GlobalConfig *Config
var configPath = "/etc/config/kuberhealthy.yaml"

// var KHClient *kubeclient.KHClient    // KubernetesClient is the global kubernetes client
// var KubeClient *kubernetes.Clientset // global k8s client used by all things
var KHController *controller.KuberhealthyCheckReconciler

func main() {

	// root context of Kuberhealthy. Revoke this and everything shuts down.
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctx.Done()

	// Initial setup before starting Kuberhealthy. Loading, parsing, and setting flags, config values and environment vars.
	err := setUp()
	if err != nil {
		log.Fatalln("startup: error setting up kuberhealthy:", err)
	}

	// setup a channel to capture when a shutdown is done
	doneChan := make(chan struct{})

	// make a new Kuberhealthy instance
	kh, err := kuberhealthy.New(ctx)
	if err != nil {
		log.Errorln("startup: failed to initalize kuberhealthy:", err)
	}

	// Make a new kubebuilder controller instance with the kuberhealthy instance in it.
	// This is will be used as a global client
	KHController, err = controller.New(ctx, kh)
	if err != nil {
		log.Errorln("startup: failed to setup kuberhealthy controller with error:", err)
	}

	// we must know when a shutdown signal is trapped or the main context has been canceled
	interruptChan := make(chan struct{})
	go listenForInterrupts(ctx, interruptChan)

	select {
	case <-ctx.Done():
		log.Infoln("shutdown: shutdown initiated due to main context cancellation")
	case <-interruptChan:
		ctxCancel() // revoke the main context
		log.Infoln("shutdown: shutdown initiated due to signal interrupt")
	}

	// once its time to shut down, we do so after the maximum timeout or when shutdown is complete gracefully
	select {
	case <-time.After(GlobalConfig.TerminationGracePeriodSeconds + (time.Second * 10)):
		log.Errorln("shutdown: shutdown took too long - exiting forcefully")
	case <-doneChan:
		log.Infoln("shutdown: shutdown completed gracefully")
	}
	return
}

// listenForInterrupts watches for termination signals and acts on them
func listenForInterrupts(ctx context.Context, interruptChan chan struct{}) {

	// register for shutdown events on sigChan
	ctx, ctxCancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer ctxCancel()

	log.Infoln("shutdown: waiting for sigChan notification...")
	<-ctx.Done()                // wait for signal to occur
	interruptChan <- struct{}{} // notify that we got a signal

}

// initConfig loads and sets default Kuberhealthy configurations
// Everytime kuberhealthy sees a configuration change, configurations should reload and reset
func initConfig() error {
	GlobalConfig = &Config{
		// kubeConfigFile: filepath.Join(os.Getenv("HOME"), ".kube", "config"),
		LogLevel: "info",
	}

	// attempt to load config file from disk
	err := GlobalConfig.Load(configPath)
	if err != nil {
		log.Println("WARNING: Failed to read configuration file from disk:", err)
	}

	// Set the target namespace to whatever the KH_TARGET_NAMESPACE env var is.  Defaults to blank, which means all, if unset.
	GlobalConfig.TargetNamespace = os.Getenv("KH_TARGET_NAMESPACE")

	// External Check URL
	// set env variables into config if specified. otherwise set external check URL to default
	checkReportURL := os.Getenv("KH_CHECK_REPORT_HOSTNAME")
	if len(checkReportURL) == 0 {
		// autoconfigure reporting URL based off of current pod namespace
		log.Infoln("KH_CHECK_REPORT_HOSTNAME environment variable not set. Using kuberhealthy.kuberhealthy.svc.cluster.local.")
	}
	GlobalConfig.checkReportURL = checkReportURL
	log.Infoln("External check reporting URL set to:", GlobalConfig.ReportingURL())

	return nil
}

// setUp loads, parses, and sets various Kuberhealthy configurations -- from flags, config values and env vars.
func setUp() error {

	// setup global config struct
	err := initConfig()
	if err != nil {
		return err
	}

	// setup flaggy
	flaggy.SetDescription("Kuberhealthy is an in-cluster synthetic health checker for Kubernetes.")
	flaggy.String(&configPath, "c", "config", "Absolute path to the kuberhealthy config file")
	flaggy.Bool(&GlobalConfig.DebugMode, "d", "debug", "Set to true to enable debug.")
	flaggy.Parse()

	// parse and set logging level
	parsedLogLevel, err := log.ParseLevel(GlobalConfig.LogLevel)
	if err != nil {
		err := fmt.Errorf("unable to parse log-level flag: %s", err)
		return err
	}

	// log to stdout and set the level to info by default
	log.SetOutput(os.Stdout)
	log.SetLevel(parsedLogLevel)
	log.Infoln("Startup Arguments:", os.Args)

	// no matter what if user has specified debug leveling, use debug leveling
	if GlobalConfig.DebugMode {
		log.Infoln("Setting debug output on because user specified flag")
		log.SetLevel(log.DebugLevel)
	}

	// init the global kubernetes client
	// integrii: Removed because we can use the global controller instance KHController for this
	// KHClient, err = kubeclient.New()
	// if err != nil {
	// 	return fmt.Errorf("Error setting up Kuberhealthy client for Kubernetes: %w", err)
	// }

	// set zap to log to stdout
	zapLogger := zap.New(zap.UseDevMode(GlobalConfig.DebugMode), zap.WriteTo(os.Stdout))
	zaplog.SetLogger(zapLogger)

	return nil
}
