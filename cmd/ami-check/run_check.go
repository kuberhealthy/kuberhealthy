package main

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"k8s.io/kops/pkg/apis/kops"
)

// runCheck runs the kops AMI check. This queries the kops state store
// S3 bucket and retrieves kops instance group information as well as
// the AWS EC2 marketplace for available AMIs. Instance group images
// are checked against the list of available AMIs and creates an error
// for each instance group that does not have an available AMI. Reports
// an error to Kuberhealthy if any errors are found. Otherwise, if all
// instance group AMIs are available, reports success.
func runCheck() {

	log.Infoln("Running check.")

	// Get a list of instance groups utilized by kops (from AWS S3).
	var kopsResult KOPSResult
	select {
	case kopsResult = <-listKopsInstanceGroups():
		// Handle errors from listing kops instance groups.
		if kopsResult.Err != nil {
			log.Errorln("failed to list kops instance groups:", kopsResult.Err.Error())
			err := fmt.Errorf("failed to list kops instance groups: %w", kopsResult.Err)
			reportErrorsToKuberhealthy([]string{err.Error()})
			return
		}
		log.Infoln("Retreived kops instance groups.")
	case <-ctx.Done():
		// If there is a context cancellation, exit the check.
		log.Infoln("Exiting check due to cancellation:", ctx.Err().Error())
		return
	}

	// Get a list of AMIs from AWS.
	var amiResult AMIResult
	select {
	case amiResult = <-listEC2Images():
		// Handle errors from listing AMIs.
		if amiResult.Err != nil {
			log.Errorln("failed to list AMIs:", amiResult.Err.Error())
			err := fmt.Errorf("failed to list AMIs: %w", amiResult.Err)
			reportErrorsToKuberhealthy([]string{err.Error()})
			return
		}
		log.Infof("Retrieved AWS AMIs. (Total: %d)", len(amiResult.Images))
	case <-ctx.Done():
		// If there is a context cancellation, exit the check.
		log.Infoln("Exiting check due to cancellation:", ctx.Err().Error())
		return
	}

	// Create a report for each instance group.
	var instanceGroupReport map[string]error
	select {
	case instanceGroupReport = <-checkIfImagesAreStillAvailable(kopsResult.InstanceGroups, amiResult.Images):
		// Handle errors from checking availability.
		if len(instanceGroupReport) != 0 {
			err := fmt.Errorf("failed AMI availability check")
			log.Errorln(err)
			errorReport := make([]string, 0)
			for _, report := range instanceGroupReport {
				errorReport = append(errorReport, report.Error())
			}
			reportErrorsToKuberhealthy(errorReport)
			return
		}
		log.Infoln("kops used images are available.")
	case <-ctx.Done():
		// If there is a context cancellation, exit the check.
		log.Infoln("Exiting check due to cancellation:", ctx.Err().Error())
		return
	}

	reportOKToKuberhealthy()
}

// checkIfImagesAreStillAvailable checks kops instance groups against a list of available
// AWS AMIs. Returns a channel of map[string]error that represents a report on each instance
// group in the kops state store. If all instance group AMIs are found, the returned map will
// have 0 keys.
func checkIfImagesAreStillAvailable(igs []*kops.InstanceGroup, images []*ec2.Image) chan map[string]error {
	// Make a channel for maps that associate instance groups with their errors.
	errorChan := make(chan map[string]error)

	go func() {
		defer close(errorChan)

		// Use a map to keep track of the instance groups that have issues.
		instanceGroupReport := make(map[string]error, 0)
		for _, ig := range igs {
			log.Infoln("Looking at instance group:", ig.Name)

			// Assume that the instance group images do not exist.
			_, ok := instanceGroupReport[ig.Name]
			if !ok {
				instanceGroupReport[ig.Name] = fmt.Errorf("could not find image matching %s", ig.Spec.Image)
			}

			// The image name is stored as owner/image.
			// We only want to check if the image exists.
			igNameSplits := strings.Split(ig.Spec.Image, "/")
			igImageName := igNameSplits[1]

			for _, image := range images {
				// Look through the list of images to see if it exists.
				if image.Name != nil {
					if strings.Contains(strings.TrimSpace(*image.Name), strings.TrimSpace(igImageName)) {
						// If the image exists, remove the assumption that the image does not exist from the map.
						log.Infoln("Found kops instance group image within list:", *image.Name)
						delete(instanceGroupReport, ig.Name)
						continue
					}
				}

				if image.ImageLocation != nil {
					if strings.Contains(strings.TrimSpace(*image.ImageLocation), strings.TrimSpace(igImageName)) {
						// If the image exists, remove the assumption that the image does not exist from the map.
						log.Infoln("Found kops instance group image wihtin list:", *image.ImageLocation)
						delete(instanceGroupReport, ig.Name)
						continue
					}
				}
			}
		}

		errorChan <- instanceGroupReport
		return
	}()

	return errorChan
}
