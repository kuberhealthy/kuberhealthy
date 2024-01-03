package main

import (
	"errors"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// getEnvVar attempts to retrieve and then validates an environmental variable
func getEnvVar(v string) (string, error) {
	var err error
	envVar := os.Getenv(v)
	if len(envVar) < 1 {
		err = errors.New("Could not retrieve Environment variable, or it had no content. " + v)
	}
	return envVar, err
}

// getAllLogLevels fetches a string list of possible log levels that can be set
func getAllLogLevels() string {
	var levelStrings []string
	for _, level := range log.AllLevels {
		levelStrings = append(levelStrings, level.String())
	}
	return strings.Join(levelStrings, ",")
}

// notifyChanLimiter takes in a chan used for notifications and smooths it out to at most
// one single notification every specified duration.  This will continuously empty whatever the inChan
// channel fed to it is.  Useful for controlling noisy upstream channel spam. This smooths notifications
// out so that outChan is only notified after inChan has been quiet for a full duration of
// the specified maxSpeed.  Stops running when inChan closes
func notifyChanLimiter(maxSpeed time.Duration, inChan chan struct{}, outChan chan struct{}) {

	// we wait for an initial inChan message and then watch for spam to stop.
	// when inChan closes, the func exits
	for range inChan {
		log.Infoln("channel notify limiter witnessed an upstream message on inChan")

		// Label for following for-select loop
	notifyChannel:
		for {
			log.Debugln("channel notify limiter waiting to receive another inChan or notify after", maxSpeed)
			select {
			case <-time.After(maxSpeed):
				log.Debugln("channel notify limiter reached", maxSpeed, ". Sending output")
				outChan <- struct{}{}
				// break out of the for-select loop and go through next inChan loop iteration if any.
				break notifyChannel
			case <-inChan:
				log.Debugln("channel notify limiter witnessed an upstream message on inChan and is waiting an additional", maxSpeed, "before sending output")
			}
		}

		log.Debugln("channel notify limiter finished going through notifications")
	}
}

// containsString returns a boolean value based on whether or not a slice of strings contains
// a string.
func containsString(s string, list []string) bool {
	for _, str := range list {
		if s == str {
			return true
		}
	}
	return false
}
