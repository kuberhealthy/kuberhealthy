package main

import (
	"context"
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

func (fc *FakeCheck) Run(ctx context.Context, c *kubernetes.Clientset) error {
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
