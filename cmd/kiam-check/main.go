package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	awsutil "github.com/Comcast/kuberhealthy/pkg/aws"
	kh "github.com/Comcast/kuberhealthy/pkg/checks/external/checkClient"
	"github.com/aws/aws-sdk-go/aws/session"
	log "github.com/sirupsen/logrus"
)

var (
	awsRegionEnv = os.Getenv("AWS_REGION")
	awsRegion    string

	useLambdasEnv = os.Getenv("LAMBDA")
	useLambdas    bool

	expectedLambdaCountEnv = os.Getenv("LAMBDA_COUNT")
	expectedLambdaCount    int

	ctx context.Context

	sess *session.Session

	signalChan chan os.Signal

	debugEnv = os.Getenv("DEBUG")
	debug    bool
)

const (
	// Default AWS region.
	// Sorry we live in US PNW :)
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

	var err error

	// Create an AWS client.
	sess = awsutil.CreateAWSSession()
	if sess == nil {
		err = fmt.Errorf("nil AWS session: %v", sess)
		reportErrorsToKuberhealthy([]string{err.Error()})
		return
	}

	go listenForInterrupts()

	switch {
	case useLambdas:
		select {
		case err = <-runLambdaCheck():
			if err != nil {
				err = fmt.Errorf("error occurred during Lambda check: %w", err)
				reportErrorsToKuberhealthy([]string{err.Error()})
				return
			}
			log.Infoln("AWS Lambda check successful.")
		case <-ctx.Done():
			return
		}
	default:
		log.Fatalln("No given AWS service to test.")
	}

	reportOKToKuberhealthy()
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
