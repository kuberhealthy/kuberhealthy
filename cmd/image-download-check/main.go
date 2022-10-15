package main

import (
	"os"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/nodeCheck"
	log "github.com/sirupsen/logrus"
)

var (

	// fullImageURL is the full registry + image name + tag URL for ease of testing
	fullImageURL = os.Getenv("FULL_IMAGE_URL")

	// timeoutLimit sets the maximum amount of time in seconds that an expected
	// image pull from a configured registry should not breach
	timeoutLimit = os.Getenv("TIMEOUT_LIMIT")
)

func init() {

	// set debug mode for nodeCheck pkg
	nodeCheck.EnableDebugOutput()

	// check to make sure fullImageURL string is provided
	if fullImageURL == "" {
		reportErrorAndStop("No FULL_IMAGE_URL string provided in YAML")
	}
}

func main() {

	var err error

	// run check
	pass := checkPass()

	// report success or failure to Kuberhealthy servers
	if pass {
		err = reportKHSuccess()
		if err != nil {
			log.Println("there was an error reporting success to KH", err)
		}
	} else {
		err = reportKHFailure("check has failed, reporting failure to KH")
		if err != nil {
			log.Println("there was an error reporting failure to KH", err)
		}
	}
}

// checkPass implements the logic to pull an image, track a start and end time, then
// determines if the actual pull time is greater than the specified timeoutLimit
func checkPass() bool {

	// initialize a start time
	startTime := time.Now()

	// download image
	img, err := downloadImage()
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("successfully downloaded image: ", img)

	// calculate time it took to complete image download
	endTime := time.Now()
	duration := endTime.Sub(startTime)
	log.Println("image took this many seconds to download: ", duration.Seconds())

	// determine if duration exceeds the time limit threshold
	timeoutLimitDuration, err := time.ParseDuration(timeoutLimit)
	if err != nil {
		log.Println("there was an error converting string of timeoutLimit to an int ", err)
	}

	// check if time duration to download image is less than our specified timeout limit
	log.Println("checking to see if", duration, "<", timeoutLimitDuration)
	if duration < timeoutLimitDuration {
		log.Println("check passes, download duration is less than timeout limit.")
		return true
	}

	log.Println("check fails, download duration is greater than timeout limit.")
	return false
}

// downloadImage pulls an image from a specified fullImageURL
func downloadImage() (v1.Image, error) {

	// pull image
	i, err := crane.Pull(fullImageURL)
	if err != nil {
		return nil, err
	}

	// save image tarball to path
	err = crane.Save(i, "emptytag", "/dev/null")
	if err != nil {
		return nil, err
	}

	// get layer count - informative
	l, err := i.Layers()
	if err != nil {
		return nil, err
	}
	log.Println("layer count", len(l))

	// get image size - informative
	s, err := i.Size()
	if err != nil {
		return nil, err
	}
	log.Println("image size", s)

	return i, nil
}

// reportKHSuccess reports success to Kuberhealthy servers and verifies the report successfully went through
func reportKHSuccess() error {
	err := checkclient.ReportSuccess()
	if err != nil {
		log.Println("Error reporting success to Kuberhealthy servers:", err)
		return err
	}
	log.Println("Successfully reported success to Kuberhealthy servers")
	return err
}

// reportKHFailure reports failure to Kuberhealthy servers and verifies the report successfully went through
func reportKHFailure(errorMessage string) error {
	err := checkclient.ReportFailure([]string{errorMessage})
	if err != nil {
		log.Println("Error reporting failure to Kuberhealthy servers:", err)
		return err
	}
	log.Println("Successfully reported failure to Kuberhealthy servers")
	return err
}

// reportErrorAndStop reports to kuberhealthy of error and exits program when called
func reportErrorAndStop(s string) {
	log.Infoln("attempting to report error to kuberhealthy:", s)
	err := checkclient.ReportFailure([]string{s})
	if err != nil {
		log.Errorln("failed to report to kuberhealthy servers:", err)
		os.Exit(1)
	}
	log.Infoln("Successfully reported to Kuberhealthy")
	os.Exit(0)
}
