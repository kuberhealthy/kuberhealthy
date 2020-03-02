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
	"fmt"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// RequestResult represents a HTTP request result.
type RequestResult struct {
	Response *http.Response
	Err      error
}

// makeRequestToDeploymentCheckService attempts to make an HTTP request to the service
// hostname and returns a boolean value. Returns a chan of errors.
func makeRequestToDeploymentCheckService(hostname string) chan error {

	requestChan := make(chan error)

	go func() {
		log.Infoln("Looking for a response from the endpoint.")

		// Init a timeout for request backoffs. Assume that this should not take more than 3m.
		backoffTimeout := time.Minute * 3
		timeoutChan := time.After(backoffTimeout)
		log.Debugln("Setting timeout for backoff loop to:", backoffTimeout)

		defer close(requestChan)

		if len(hostname) == 0 {
			err := fmt.Errorf("given blank hostname for service load balancer endpoint -- skipping HTTP call")
			requestChan <- err
			return
		}

		// Prepend the hostname with a HTTP protocol.
		if !strings.HasPrefix(hostname, "http://") {
			hostname = "http://" + hostname
		}

		// Try to make requests to the hostname endpoint and wait for a result.
		select {
		case result := <-getRequestBackoff(hostname):
			if &result == nil {
				requestChan <- errors.New("got a nil request result from the backoff process")
				return
			}

			if result.Err != nil {
				requestChan <- result.Err
				return
			}

			if result.Response == nil {
				err := fmt.Errorf("could not get a response from the given address: %s", hostname)
				requestChan <- err
				return
			}

			if result.Response.StatusCode != http.StatusOK {
				requestChan <- result.Err
				return
			}

			log.Infoln("Got a result from", http.MethodGet, "request backoff:", result.Response.Status)
			requestChan <- nil
		case <-timeoutChan:
			log.Errorln("Backoff loop for a 200 response took too long and timed out.")
			err := cleanUp(ctx)
			if err != nil {
				err = fmt.Errorf("failed to clean up properly: %w", err)
			}
			requestChan <- err
		case <-ctx.Done():
			log.Errorln("Context expired while waiting for a", http.StatusOK, "from "+hostname+".")
			err := cleanUp(ctx)
			if err != nil {
				err = fmt.Errorf("failed to clean up properly: %w", err)
			}
			requestChan <- err
		}

	}()

	return requestChan
}

// getRequestBackoff returns a channel that reports the result of a GET backoff loop from the hostname endpoint.
func getRequestBackoff(hostname string) chan RequestResult {

	requestResultChan := make(chan RequestResult)

	// Make requests to the hostname endpoint in the background.
	go func() {

		defer close(requestResultChan)

		requestResult := RequestResult{}

		// Backoff time = attempts * 5 seconds.
		retrySleep := func(t int) {
			log.Infoln("Retrying in", t*5, "seconds.")
			time.Sleep(time.Duration(t) * 5 * time.Second)
		}

		// Backoff loop here for HTTP GET request.
		attempts := 1
		maxRetries := 10
		log.Infoln("Beginning backoff loop for HTTP", http.MethodGet, "request.")
		err := errors.New("") // Set err to something that is not nil to start the following loop.
		for err != nil {      // Loop on http.Get() errors.
			if attempts > maxRetries {
				log.Infoln("Could not successfully make an HTTP request after", attempts, "attempts.")
				requestResult.Err = err
				requestResultChan <- requestResult
				return
			}

			log.Debugln("Making", http.MethodGet, "to", hostname)
			var resp *http.Response
			resp, err = http.Get(hostname)
			if err == nil && resp.StatusCode == http.StatusOK {
				log.Infoln("Successfully made an HTTP request on attempt:", attempts)
				log.Infoln("Got a", resp.StatusCode, "with a", http.MethodGet, "to", hostname)
				resp.Body.Close()
				requestResult.Response = resp
				requestResultChan <- requestResult
				return
			}

			if !strings.Contains(err.Error(), "no such host") {
				log.Debugln("An error occurred making a", http.MethodGet, "request:", err)
			}

			if resp != nil {
				log.Debugln("Got a", resp.StatusCode)
				resp.Body.Close()
			}

			retrySleep(attempts)
			attempts++
		}
		if err != nil {
			log.Errorln("Could not make a", http.MethodGet, "request to", hostname, "due to:", err.Error())
			requestResult.Err = fmt.Errorf("failed to hit endpoint after backoff loop: %w", err)
		}

		requestResultChan <- requestResult
		return
	}()

	return requestResultChan
}
