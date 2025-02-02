// Kuberhealthy is an enhanced health check for Kubernetes clusters.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/integrii/flaggy"
	log "github.com/sirupsen/logrus"
	manager "sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kuberhealthy/kuberhealthy/v3/pkg/kubeclient"
	"github.com/kuberhealthy/kuberhealthy/v3/pkg/masterCalculation"
)

// GlobalConfig holds the configuration settings for Kuberhealthy
var GlobalConfig *Config
var configPath = "/etc/config/kuberhealthy.yaml"

const KHCheckNameAnnotationKey = "comcast.github.io/check-name" // KHCheckNameAnnotationKey is the key used in the annotation that holds the check's short name
const KHExternalReportingURL = "KH_EXTERNAL_REPORTING_URL"      // KHExternalReportingURL is the environment variable key used to override the URL checks will be asked to report in to
const DefaultRunInterval = time.Minute * 10                     // DefaultRunInterval is the default run interval for checks set by kuberhealthy

var KubernetesClient *kubeclient.KHClient                           // KubernetesClient is the global kubernetes client
var CRDManager manager.Manager                                      // CRDManager holds the event handlers for CRDs as well as the Kuberhealthy CRD Clients
var podNamespace = os.Getenv("POD_NAMESPACE")                       // the namespace the pod runs in
var checkReaperRunInterval = os.Getenv("CHECK_REAPER_RUN_INTERVAL") // Interval for how often check pods should get reaped. Default is 30s.
var podHostname string                                              // the hostname of this pod
var isMaster bool                                                   // indicates this instance is the master and should be running checks
var upcomingMasterState bool                                        // the upcoming master state on next interval
var lastMasterChangeTime time.Time                                  // indicates the last time a master change was seen
var terminationGracePeriod = time.Minute * 5                        // keep calibrated with kubernetes terminationGracePeriodSeconds
var DefaultTimeout = time.Minute * 5                                // DefaultTimeout is the default timeout for external checks

func main() {

	ctx := context.Background()

	// Initial setup before starting Kuberhealthy. Loading, parsing, and setting flags, config values and environment vars.
	err := setUp()
	if err != nil {
		log.Fatalln("Error setting up Kuberhealthy:", err)
	}

	// init the global kubernetes client
	KubernetesClient, err = kubeclient.New()
	if err != nil {
		log.Fatalln("Error setting up Kuberhealthy client for Kubernetes:", err)
	}

	KubernetesClient.GetKuberhealthyState("test", "default")
	// init the CRD manager
	CRDManager, err = newCRDManager()
	if err != nil {
		log.Fatalln("Error setting up Kuberhealthy CRD manager:", err)
	}

	// start the CRD manager
	err = CRDManager.Start(ctx)
	if err != nil {
		log.Fatalln("Error starting CRD manager:", err)
	}

	// Create a new Kuberhealthy struct
	kuberhealthy := NewKuberhealthy(GlobalConfig)

	// create run context and start listening for shutdown interrupts
	khRunCtx, khRunCtxCancelFunc := context.WithCancel(context.Background())
	kuberhealthy.shutdownCtxFunc = khRunCtxCancelFunc // load the KH struct with a func to shutdown its control system
	go listenForInterrupts(kuberhealthy)

	// tell Kuberhealthy to start all checks and master change monitoring
	kuberhealthy.Start(khRunCtx)

	time.Sleep(time.Second * 90) // give the interrupt handler a period of time to call exit before we shutdown
	<-time.After(terminationGracePeriod + (time.Second * 10))
	log.Errorln("shutdown: main loop was ready for shutdown for too long. exiting.")
	os.Exit(1)
}

// listenForInterrupts watches for termination signals and acts on them
func listenForInterrupts(k *Kuberhealthy) {
	// shutdown signal handling
	sigChan := make(chan os.Signal, 1)

	// register for shutdown events on sigChan
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	log.Infoln("shutdown: waiting for sigChan notification...")
	<-sigChan
	log.Infoln("shutdown: Shutting down due to sigChan signal...")

	// wait for check to fully shutdown before exiting
	doneChan := make(chan struct{})
	go k.Shutdown(doneChan)

	// wait for checks to be done shutting down before exiting
	select {
	case <-doneChan:
		log.Infoln("shutdown: Shutdown gracefully completed!")
		log.Infoln("shutdown: exiting 0")
		os.Exit(0)
	case <-sigChan:
		log.Warningln("shutdown: Shutdown forced from multiple interrupts!")
		log.Infoln("shutdown: exiting 1")
		os.Exit(1)
	case <-time.After(terminationGracePeriod):
		log.Errorln("shutdown: Shutdown took too long.  Shutting down forcefully!")
		log.Infoln("shutdown: exiting 1")
		os.Exit(1)
	}
}

// setUpConfig loads and sets default Kuberhealthy configurations
// Everytime kuberhealthy sees a configuration change, configurations should reload and reset
func setUpConfig() error {
	GlobalConfig = &Config{
		kubeConfigFile: filepath.Join(os.Getenv("HOME"), ".kube", "config"),
		LogLevel:       "info",
	}

	// attempt to load config file from disk
	err := GlobalConfig.Load(configPath)
	if err != nil {
		log.Println("WARNING: Failed to read configuration file from disk:", err)
	}

	// set env variables into config if specified. otherwise set external check URL to default
	externalCheckURL, err := getEnvVar(KHExternalReportingURL)
	if err != nil {
		if len(podNamespace) == 0 {
			return errors.New("env KH_EXTERNAL_REPORTING_URL not set and POD_NAMESPACE environment variable was blank")
		}
		log.Infoln("KH_EXTERNAL_REPORTING_URL environment variable not set, using default value")
		externalCheckURL = "http://kuberhealthy." + podNamespace + ".svc.cluster.local/externalCheckStatus"
	}
	GlobalConfig.ExternalCheckReportingURL = externalCheckURL
	log.Infoln("External check reporting URL set to:", GlobalConfig.ExternalCheckReportingURL)
	return nil
}

// setUp loads, parses, and sets various Kuberhealthy configurations -- from flags, config values and env vars.
func setUp() error {

	var useDebugMode bool

	// setup global config struct
	err := setUpConfig()
	if err != nil {
		return err
	}

	// setup flaggy
	flaggy.SetDescription("Kuberhealthy is an in-cluster synthetic health checker for Kubernetes.")
	flaggy.String(&configPath, "c", "config", "Absolute path to the kuberhealthy config file")
	flaggy.Bool(&useDebugMode, "d", "debug", "Set to true to enable debug.")
	flaggy.Bool(&GlobalConfig.EnableForceMaster, "", "forceMaster", "Set to force master responsibilities on.")
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
	if useDebugMode {
		log.Infoln("Setting debug output on because user specified flag")
		log.SetLevel(log.DebugLevel)
	}

	// Handle force master mode
	if GlobalConfig.EnableForceMaster {
		log.Infoln("Enabling forced master mode")
		masterCalculation.DebugAlwaysMasterOn()
	}

	// determine the name of this pod from the POD_NAME environment variable
	podHostname, err = getEnvVar("POD_NAME")
	if err != nil {
		err := fmt.Errorf("failed to determine my hostname: %s", err)
		return err
	}

	// determine the name of this pod from the POD_NAME environment variable
	GlobalConfig.TargetNamespace = os.Getenv("TARGET_NAMESPACE")

	return nil
}
