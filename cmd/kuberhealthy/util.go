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
// one single notification every 10 seconds.  This will continuously empty whatever the "in"
// channel fed to it is.  Useful for controlling noisy upstream channel spam.  Stops when
// the upstream inChan channel closes.  Closes outChan when completed
func notifyChanLimiter(maxSpeed time.Duration, inChan chan struct{}, outChan chan struct{}) {

	ticker := time.NewTicker(maxSpeed)
	defer ticker.Stop()
	var upstreamClosed bool // indicates its time to stop reading

	// on every tick, we read all incoming messages and notify if we found any
	for range ticker.C {

		var gotMessage bool
		var doneReading bool

		// read all incoming stuff from inChan until the channel is empty
		for {
			if doneReading {
				break
			}
			select {
			case _, closed := <-inChan:
				log.Println("channel notify limiter witnessed an upstream message on inChan")
				gotMessage = true
				if closed {
					upstreamClosed = true
				}
			default:
				doneReading = true
			}
		}

		// if inChan had a message, we send a message to outChan
		if gotMessage {
			log.Println("channel notify limiter sending a message on outChan")
			outChan <- struct{}{}
		}

		// if the upstream inChan closes, close the downsteram and exit
		if upstreamClosed {
			close(outChan)
			return
		}
	}

}
