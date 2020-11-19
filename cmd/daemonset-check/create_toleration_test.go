package main

import (
	"testing"
	corev1 "k8s.io/api/core/v1"
)

func TestCreateToleration(test *testing.T) {
	//string_tol := ""
	//string_tol := "blah"
	string_tol := "=test"
	//string_tol := "blah=test:NoSchedule"
	
	//nodeSelectors := map[string]string{}
	expectedResults := corev1.Toleration {
		Key: "blah",
	//	Value: "test",
	//	Effect: corev1.TaintEffect("NoSchedule"),
		Operator: corev1.TolerationOpExists,
	}
	//expectedResults := corev1.Toleration{}
	test.Log("testing createToleration")
	r, err := createToleration(string_tol)
	if err != nil {
		test.Errorf("%v", err)
	} else if *r != expectedResults {
		test.Errorf("Expected %+v got %+v", expectedResults, r)	
	}
}
