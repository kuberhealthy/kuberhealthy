package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	awsutil "github.com/Comcast/kuberhealthy/pkg/aws"
	kh "github.com/Comcast/kuberhealthy/pkg/checks/external/checkClient"
	"github.com/aws/aws-sdk-go/aws/session"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
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
		if len(awsS3BucketName) == 0 {
			log.Fatalln("Given AWS_S3_BUCKET_NAME is empty:", awsS3BucketName)
		}
	}

	// Parse CLUSTER_FQDN environment variable.
	clusterName = defaultClusterName
	if len(clusterNameEnv) != 0 {
		clusterName = clusterNameEnv
		if len(clusterName) == 0 {
			log.Fatalln("Given CLUSTER_FQDN is empty:", clusterName)
		}
	}

	// Create a signal chan for interrupts.
	signalChan = make(chan os.Signal, 2)

	// Create a context for this check.
	ctx = context.Background()

	var err error
	if len(debugEnv) != 0 {
		debug, err = strconv.ParseBool(debugEnv)
		if err != nil {
			log.Fatalln("Unable to parse DEBUG environment variable:", err)
		}
	}

	if debug {
		log.SetLevel(log.DebugLevel)
		log.Infoln("Debug logging enabled")
	}
	log.Debugln(os.Args)
}

func main() {

	var err error

	// Create an AWS session.
	awsSess = awsutil.CreateAWSSession()
	if awsSess == nil {
		err = fmt.Errorf("nil AWS session")
		reportErrorsToKuberhealthy([]string{err.Error()})
		log.Fatalln("AWS session is nil:", awsSess)
	}

	// Start listening for interrupts in the background.
	go listenForInterrupts()

	// Run the check.
	runCheck()

	os.Exit(0)
}

// listenForInterrupts watches the signal and done channels for termination.
func listenForInterrupts() {

	<-signalChan // This is a blocking operation -- the routine will stop here until there is something sent down the channel.
	log.Infoln("Received an interrupt signal from the signal channel.")

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
	var attempts int
	var err error

	retry := func() {
		log.Infoln("Retrying a report to Kuberhealthy in 5 seconds.")
		time.Sleep(time.Second * 5)
	}

	// Keep retrying until it works.
	for {
		attempts++
		log.Infoln("Reporting status to Kuberhealthy:", ok)

		if attempts > 1 {
			log.Infoln("Attempt", attempts, "reporting status to Kuberhealthy.")
		}

		if attempts > 3 {
			log.Infoln("Exiting retry loop.")
			return
		}

		if ok {
			err = kh.ReportSuccess()
			if err != nil {
				retry()
				continue
			}
			return
		}
		err = kh.ReportFailure(errs)
		if err != nil {
			retry()
			continue
		}
		return
	}
}
