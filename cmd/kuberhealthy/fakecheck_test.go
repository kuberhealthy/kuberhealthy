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
	"errors"
	"time"

	"k8s.io/client-go/kubernetes"
)

// FakeCheck implements the kuberhealthy check interface with a fake
// checker to be used for testing
type FakeCheck struct {
	OK                      bool
	Errors                  []string
	ShouldHaveRunError      bool          // when set to true, runs will return errors
	ShouldHaveShutdownError bool          // when set to true, shutdowns will return errors
	IntervalValue           time.Duration // the value we should return when Interval() is called
	FakeError               string        // the string thrown when ShouldHaveRunError or ShouldHaveShutdownError is set to true and Shutdown or Run is called
	CheckName               string        // the name of this check
	Namespace               string        // the namespace of the fake check
}

func (fc *FakeCheck) Name() string {
	return fc.CheckName
}

func (fc *FakeCheck) CheckNamespace() string {
	return fc.Namespace
}

func (fc *FakeCheck) Interval() time.Duration {
	return fc.IntervalValue
}

func (fc *FakeCheck) Timeout() time.Duration {
	return time.Minute
}

func (fc *FakeCheck) CurrentStatus() (bool, []string) {
	return fc.OK, fc.Errors
}

func (fc *FakeCheck) Run(c *kubernetes.Clientset) error {
	if fc.ShouldHaveRunError {
		return errors.New(fc.FakeError)
	}
	return nil
}

func (fc *FakeCheck) Shutdown() error {
	if fc.ShouldHaveShutdownError {
		return errors.New(fc.FakeError)
	}
	return nil
}

func NewFakeCheck() *FakeCheck {
	fc := FakeCheck{}
	fc.OK = true
	fc.IntervalValue = time.Second
	fc.Errors = []string{}
	fc.FakeError = "FakeCheck Error"
	fc.CheckName = "FakeCheck"
	return &fc
}
