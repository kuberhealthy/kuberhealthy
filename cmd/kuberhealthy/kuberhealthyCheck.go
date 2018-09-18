/* Copyright 2018 Comcast Cable Communications Management, LLC
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at
       http://www.apache.org/licenses/LICENSE-2.0
   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/
package main

import (
	"time"

	"k8s.io/client-go/kubernetes"
)

// KuberhealthyCheck represents the required methods for a check to be ran by
// kuberhealthy.
type KuberhealthyCheck interface {
	// Name returns the identfier for this check such as: Checker
	Name() string
	// CheckNamespace returns the name of the namespace that the check runs in
	CheckNamespace() string
	// Interval returns a run interval indicating how often this check
	// should be performed
	Interval() time.Duration
	// Timeout returns a duration indicating how long we should wait for
	// this check to run
	Timeout() time.Duration
	// CurrentStatus returns the current status of the check and its
	// error messages.  The bool indicates health. (true = up and false = down).
	// This function should not do anything complex and is expected to return
	// quickly.  It will be invoked on every status page request.
	CurrentStatus() (bool, []string)
	// Run fires off a single check.  It is invoked each time the Interval
	// ticker ticks.  Results of the error are stored within the check
	// and not tracked from the upstream worker that ticks.  Results should
	// show up when CurrentStatus() is invoked.
	Run(c *kubernetes.Clientset) error
	// Shutdown is called when Kuberhealthy needs to close.  The check has up
	// to 30 seconds to clean up anything in progress and begin shutdown.
	// When the check completes and returns, we assume it is done shutting
	// down.
	Shutdown() error
}
