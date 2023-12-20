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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	khcheckv1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khcheck/v1"
	khjobv1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khjob/v1"
	khstatev1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khstate/v1"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/kubeClient"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/masterCalculation"
)

// status represents the current Kuberhealthy OK:Error state
var cfg *Config
var configPath = "/etc/config/kuberhealthy.yaml"

var podNamespace = os.Getenv("POD_NAMESPACE")
var isMaster bool                  // indicates this instance is the master and should be running checks
var upcomingMasterState bool       // the upcoming master state on next interval
var lastMasterChangeTime time.Time // indicates the last time a master change was seen
// Interval for how often check pods should get reaped. Default is 30s.
var checkReaperRunInterval = os.Getenv("CHECK_REAPER_RUN_INTERVAL")

var terminationGracePeriod = time.Minute * 5 // keep calibrated with kubernetes terminationGracePeriodSeconds

// the hostname of this pod
var podHostname string

// KHExternalReportingURL is the environment variable key used to override the URL checks will be asked to report in to
const KHExternalReportingURL = "KH_EXTERNAL_REPORTING_URL"

// DefaultRunInterval is the default run interval for checks set by kuberhealthy
const DefaultRunInterval = time.Minute * 10

// DefaultTimeout is the default timeout for external checks
var DefaultTimeout = time.Minute * 5

// KHCheckNameAnnotationKey is the key used in the annotation that holds the check's short name
const KHCheckNameAnnotationKey = "comcast.github.io/check-name"

// khCheckClient is a client for khstate custom resources
var khStateClient *khstatev1.KHStateV1Client

// khStateClient is a client for khcheck custom resources
var khCheckClient *khcheckv1.KHCheckV1Client

// khJobClient is a client for khjob custom resources
var khJobClient *khjobv1.KHJobV1Client

// constants for using the kuberhealthy status CRD
// const stateCRDGroup = "comcast.github.io"
// const stateCRDVersion = "v1"
const stateCRDResource = "khstates"

// constants for using the kuberhealthy check CRD
const checkCRDGroup = "comcast.github.io"
const checkCRDVersion = "v1"
const checkCRDResource = "khchecks"

// kubernetesClient is the global kubernetes client
var kubernetesClient *kubernetes.Clientset

// dynamicClient represents the client used to watch and list unstructured khchecks
var dynamicClient dynamic.Interface

func main() {

	// Initial setup before starting Kuberhealthy. Loading, parsing, and setting flags, config values and environment vars.
	err := setUp()
	if err != nil {
		log.Fatalln("Error setting up Kuberhealthy:", err)
	}

	// Create a new Kuberhealthy struct
	kuberhealthy := NewKuberhealthy(cfg)

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

// initKubernetesClients creates the appropriate CRD clients and kubernetes client to be used in all cases. Issue #181
func initKubernetesClients() error {

	// make a new kuberhealthy client
	kc, err := kubeClient.Create(cfg.kubeConfigFile)
	if err != nil {
		return err
	}
	kubernetesClient = kc

	// make a new crd check client
	checkClient, err := khcheckv1.Client(cfg.kubeConfigFile)
	if err != nil {
		return err
	}
	khCheckClient = checkClient

	// make a new crd state client
	stateClient, err := khstatev1.Client(cfg.kubeConfigFile)
	if err != nil {
		return err
	}
	khStateClient = stateClient

	// make a new crd job client
	jobClient, err := khjobv1.Client(cfg.kubeConfigFile)
	if err != nil {
		return err
	}
	khJobClient = jobClient

	// make a dynamicClient for kubernetes unstructured checks
	restConfig, err := clientcmd.BuildConfigFromFlags(kc.RESTClient().Get().URL().Host, configPath)
	if err != nil {
		log.Fatalln("Failed to build kubernetes configuration from configuration flags:", err)
	}

	dynamicClient, err = dynamic.NewForConfig(restConfig)
	if err != nil {
		log.Fatalln("Failed to create kubernetes dynamic client configuration")
	}

	return nil
}

// setUpConfig loads and sets default Kuberhealthy configurations
// Everytime kuberhealthy sees a configuration change, configurations should reload and reset
func setUpConfig() error {
	cfg = &Config{
		kubeConfigFile: filepath.Join(os.Getenv("HOME"), ".kube", "config"),
		LogLevel:       "info",
	}

	// attempt to load config file from disk
	err := cfg.Load(configPath)
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
	cfg.ExternalCheckReportingURL = externalCheckURL
	log.Infoln("External check reporting URL set to:", cfg.ExternalCheckReportingURL)
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
	flaggy.Bool(&cfg.EnableForceMaster, "", "forceMaster", "Set to force master responsibilities on.")
	flaggy.Parse()

	// parse and set logging level
	parsedLogLevel, err := log.ParseLevel(cfg.LogLevel)
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
	if cfg.EnableForceMaster {
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
	cfg.TargetNamespace = os.Getenv("TARGET_NAMESPACE")

	// setup all clients
	err = initKubernetesClients()
	if err != nil {
		err := fmt.Errorf("failed to bootstrap kubernetes clients: %s", err)
		return err
	}

	return nil
}
