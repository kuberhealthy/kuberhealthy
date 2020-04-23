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
// one single notification every 10 seconds.  This will continuously empty whatever the inChan
// channel fed to it is.  Useful for controlling noisy upstream channel spam. Also smooths notifications
// out so that an outChan signal is only sent after inChan has been quiet for a full duration of
// the specified maxSpeed.  Never stops running and does not expect upstream channels from closing
func notifyChanLimiter(maxSpeed time.Duration, inChan chan struct{}, outChan chan struct{}) {

	ticker := time.NewTicker(maxSpeed)
	defer ticker.Stop()
	var mostRecentChangeTime time.Time
	var changePending bool

	// on every tick, we read all incoming messages and notify if we found any
	for range ticker.C {
		var doneReading bool // indicates that the for loop should break because the input chan is drained

		// read all incoming stuff from inChan until the channel is empty
		for {
			if doneReading {
				break
			}
			select {
			case <-inChan:
				log.Println("channel notify limiter witnessed an upstream message on inChan")
				changePending = true
				mostRecentChangeTime = time.Now()
			default:
				doneReading = true
			}
		}

		// if a change is pending for outChan, be sure that its been enough time since a change was seen and
		// send the outChan message
		if changePending {
			// if a change has come in within the last tick, then skip this run
			if time.Now().Sub(mostRecentChangeTime) < maxSpeed {
				log.Println("channel notify limiter waiting for changes to calm...")
				continue
			}

			// if it has been sufficient time since the last inChan message, send a message out outChan
			log.Println("channel notify limiter sending a message on outChan")
			outChan <- struct{}{}
			changePending = false
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
