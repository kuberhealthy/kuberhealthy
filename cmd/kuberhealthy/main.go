// Kuberhealthy is an enhanced health check for Kubernetes clusters.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	logruslogr "github.com/kuberhealthy/kuberhealthy/v3/pkg/logruslogr"
	log "github.com/sirupsen/logrus"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"github.com/kuberhealthy/kuberhealthy/v3/internal/kuberhealthy"
	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
)

// GlobalConfig holds the configuration settings for Kuberhealthy
var GlobalConfig *Config

// var KHClient *kubeclient.KHClient    // KubernetesClient is the global kubernetes client
// var KubeClient *kubernetes.Clientset // global k8s client used by all things
var Globals struct {
	kubeConfig *rest.Config
	kubeClient kubernetes.Interface
	khClient   client.Client
	kh         *kuberhealthy.Kuberhealthy
}

// GitRevision holds the source control reference embedded at build time.
// It can be overridden via ldflags: -X main.GitRevision=<ref>.
// When empty, it is set to "EXPERIMENTAL" during startup.
var GitRevision string

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

	// Build the Kubernetes config and clients once for reuse
	Globals.kubeConfig, err = ctrl.GetConfig()
	if err != nil {
		log.Fatalln("startup: failed to get kubernetes config:", err)
	}
	Globals.kubeClient, err = kubernetes.NewForConfig(Globals.kubeConfig)
	if err != nil {
		log.Fatalln("startup: failed to create kubernetes client:", err)
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(khapi.AddToScheme(scheme))
	Globals.khClient, err = client.New(Globals.kubeConfig, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalln("startup: failed to create controller-runtime client:", err)
	}

	Globals.kh = kuberhealthy.New(ctx, Globals.khClient, doneChan)
	Globals.kh.SetReportingURL(GlobalConfig.ReportingURL())
	if err := Globals.kh.Start(ctx, Globals.kubeConfig); err != nil {
		log.Errorln("startup: failed to start kuberhealthy:", err)
	}

	// start the web status server
	go func() {
		if err := StartWebServer(); err != nil {
			log.Errorln("web server error:", err)
		}
	}()

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
	GlobalConfig = New()
	if err := GlobalConfig.LoadFromEnv(); err != nil {
		return err
	}
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

	// set controller-runtime to use logrus
	logf.SetLogger(logruslogr.New(log.StandardLogger()))

	// ensure GitRevision is present and log it for traceability
	if strings.TrimSpace(GitRevision) == "" {
		GitRevision = "EXPERIMENTAL"
	}
	log.Infoln("Build Git Revision:", GitRevision)

	if os.Getenv("POD_NAME") == "" {
		host := GetMyHostname("kuberhealthy")
		os.Setenv("POD_NAME", host)
		log.Warnln("POD_NAME environment variable not set. Using", host)
	}

	// init the global kubernetes client
	// KHClient, err = kubeclient.New()
	// if err != nil {
	// 	return fmt.Errorf("Error setting up Kuberhealthy client for Kubernetes: %w", err)
	// }

	return nil
}
