package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	awsutil "github.com/kuberhealthy/kuberhealthy/v2/pkg/aws"
	kh "github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/nodeCheck"
)

var (
	// AWS region variables.
	awsRegionEnv = os.Getenv("AWS_REGION")
	awsRegion    string

	// AWS S3 Bucket variables.
	awsS3BucketNameEnv = os.Getenv("AWS_S3_BUCKET_NAME")
	awsS3BucketName    string

	// Name of the cluster for Kops state store.
	clusterNameEnv = os.Getenv("CLUSTER_FQDN")
	clusterName    string

	signalChan chan os.Signal

	debugEnv = os.Getenv("DEBUG")
	debug    bool

	ctx context.Context

	// AWS session.
	awsSess *session.Session

	// K8s client.
	k8sClient *kubernetes.Clientset
)

const (
	// Matching strings for kops bucket operations.
	regexpAWSRegion                 = `^[\w]{2}[-][\w]{4,9}[-][\d]$`
	regexpKopsStateStoreS3ObjectKey = `/instancegroup/`

	// Default AWS region.
	defaultAWSRegion = "us-east-1"

	// Default AWS S3 bucket name.
	defaultAWSS3BucketName = "kops-state-store"

	// Default cluster FQDN
	defaultClusterName = "cluster-fqdn"
)

func init() {

	// Parse AWS_REGION environment variable.
	awsRegion = defaultAWSRegion
	if len(awsRegionEnv) != 0 {
		awsRegion = awsRegionEnv
		ok, err := regexp.Match(regexpAWSRegion, []byte(awsRegion))
		if err != nil {
			log.Fatalln("Failed to parse AWS_REGION:", err.Error())
		}
		if !ok {
			log.Fatalln("Given AWS_REGION does not match AWS Region format.")
		}
	}

	// Parse AWS_S3_BUCKET_NAME environment variable.
	awsS3BucketName = defaultAWSS3BucketName
	if len(awsS3BucketNameEnv) != 0 {
		awsS3BucketName = awsS3BucketNameEnv
	}

	// Parse CLUSTER_FQDN environment variable.
	clusterName = defaultClusterName
	if len(clusterNameEnv) != 0 {
		clusterName = clusterNameEnv
	}

	// Create a signal chan for interrupts.
	signalChan = make(chan os.Signal, 2)

	// Create a context for this check.
	ctx = context.Background()

	// var err error
	if len(debugEnv) != 0 {
		// avoid the strconv package because it causes a segfault when building for multi-arch on docker. yes, really.
		debugEnv = strings.ToLower(debugEnv)
		if debugEnv == "t" || debugEnv == "true" || debugEnv == "yes" {
			debug = true
		}
		// debug, err = strconv.ParseBool(debugEnv)
		// if err != nil {
		// 	log.Fatalln("Unable to parse DEBUG environment variable:", err)
		// }
	}

	if debug {
		log.SetLevel(log.DebugLevel)
		log.Infoln("Debug logging enabled")
	}
	log.Debugln(os.Args)
}

func main() {

	var err error

	// create context
	checkTimeLimit := time.Minute * 1
	ctx, ctxCancel := context.WithTimeout(context.Background(), checkTimeLimit)
	defer ctxCancel()

	// hits kuberhealthy endpoint to see if node is ready
	err = nodeCheck.WaitForKuberhealthy(ctx)
	if err != nil {
		log.Errorln("Error waiting for kuberhealthy endpoint to be contactable by checker pod with error:" + err.Error())
	}

	// Create an AWS session.
	awsSess = awsutil.CreateAWSSession()
	if awsSess == nil {
		err = fmt.Errorf("nil AWS session")
		reportErrorsToKuberhealthy([]string{err.Error()})
		log.Fatalln("AWS session is nil:", awsSess)
	}

	// Start listening for interrupts in the background.
	go listenForInterrupts()

	// Catch panics.
	var r interface{}
	defer func() {
		r = recover()
		if r != nil {
			log.Infoln("Recovered panic:", r)
			reportToKuberhealthy(false, []string{r.(string)})
		}
	}()

	// Run the check.
	runCheck()
}

// listenForInterrupts watches the signal and done channels for termination.
func listenForInterrupts() {

	// Relay incoming OS interrupt signals to the signalChan.
	signal.Notify(signalChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)
	sig := <-signalChan // This is a blocking operation -- the routine will stop here until there is something sent down the channel.
	log.Infoln("Received an interrupt signal from the signal channel.")
	log.Debugln("Signal received was:", sig.String())

	// Clean up pods here.
	log.Infoln("Shutting down.")

	os.Exit(0)
}

// reportErrorsToKuberhealthy reports the specified errors for this check run.
func reportErrorsToKuberhealthy(errs []string) {
	log.Errorln("Reporting errors to Kuberhealthy:", errs)
	reportToKuberhealthy(false, errs)
}

// reportOKToKuberhealthy reports that there were no errors on this check run to Kuberhealthy.
func reportOKToKuberhealthy() {
	log.Infoln("Reporting success to Kuberhealthy.")
	reportToKuberhealthy(true, []string{})
}

// reportToKuberhealthy reports the check status to Kuberhealthy.
func reportToKuberhealthy(ok bool, errs []string) {
	var err error
	if ok {
		err = kh.ReportSuccess()
		if err != nil {
			log.Fatalln("error reporting to kuberhealthy:", err.Error())
		}
		return
	}
	err = kh.ReportFailure(errs)
	if err != nil {
		log.Fatalln("error reporting to kuberhealthy:", err.Error())
	}
	return
}
