package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	awsutil "github.com/kuberhealthy/kuberhealthy/v2/pkg/aws"
	kh "github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
	log "github.com/sirupsen/logrus"
)

var (
	// AWS region to query Lambdas from.
	awsRegionEnv = os.Getenv("AWS_REGION")
	awsRegion    string

	// Expected AWS Lambda count.
	expectedLambdaCountEnv = os.Getenv("LAMBDA_COUNT")
	expectedLambdaCount    int

	ctx context.Context

	// AWS session.
	sess *session.Session

	// Channel for interrupt signals.
	signalChan chan os.Signal

	debugEnv = os.Getenv("DEBUG")
	debug    bool
)

const (
	// Default AWS region.
	defaultAWSRegion = "us-west-2" // Default is Oregon.
)

func init() {

	// Parse incoming debug settings.
	parseDebugSettings()

	// Parse all incoming input environment variables and crash if an error occurs
	// during parsing process.
	parseInputValues()

	ctx = context.Background()
	signalChan = make(chan os.Signal, 2)

	// Relay incoming OS interrupt signals to the signalChan.
	signal.Notify(signalChan, os.Interrupt, os.Kill)
}

func main() {

	// Give the k8s API enough time to allocate IPs.
	time.Sleep(15)

	var err error

	// Create an AWS client.
	sess = awsutil.CreateAWSSession()
	if sess == nil {
		err = fmt.Errorf("nil AWS session: %v", sess)
		reportErrorsToKuberhealthy([]string{err.Error()})
		return
	}

	// Start listening for interrupts.
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

	// Run the Lambda list check.
	select {
	case err = <-runLambdaCheck():
		if err != nil {
			// Report a failure if there an error occurred during the check.
			err = fmt.Errorf("error occurred during Lambda check: %w", err)
			reportErrorsToKuberhealthy([]string{err.Error()})
			return
		}
		log.Infoln("AWS Lambda check successful.")
	case <-ctx.Done():
		return
	}

	reportOKToKuberhealthy()
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
