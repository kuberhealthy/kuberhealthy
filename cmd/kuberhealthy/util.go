// Copyright 2018 Comcast Cable Communications Management, LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

// getAllLogLevel fetches a string list of possible log levels that can be set
func getAllLogLevel() string {
	var levelStrings []string
	for _, level := range log.AllLevels {
		levelStrings = append(levelStrings, level.String())
	}
	return strings.Join(levelStrings, ",")
}

// notifyChanLimiter takes in a chan used for notifications and smooths it out to at most
// one single notification every specifid duration.  This will continuously empty whatever the inChan
// channel fed to it is.  Useful for controlling noisy upstream channel spam. This smooths notifications
// out so that outChan is only notified after inChan has been quiet for a full duration of
// the specified maxSpeed.  Stops running when inChan closes
func notifyChanLimiter(maxSpeed time.Duration, inChan chan struct{}, outChan chan struct{}) {

	// we wait for an initial inChan message and then watch for spam to stop.
	// when inChan closes, the func exits
	for range inChan {
		log.Println("channel notify limiter witnessed an upstream message on inChan")
		for {
			select {
			case <-time.After(maxSpeed):
				outChan <- struct{}{}
			case <-inChan:
				log.Println("channel notify limiter witnessed an upstream message on inChan and is waiting an additional", maxSpeed, "before sending output")
			}
		}
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
