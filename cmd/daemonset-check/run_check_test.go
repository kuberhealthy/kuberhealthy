package main

import (
	"testing"
)

func TestNodeLabelsMatch(test *testing.T) {
	labels := map[string]string{
		"blah":                   "blerp",
		"kubernetes.io/hostname": "ip-10-112-79-36.us-west-2.compute.internal",
		"kubernetes.io/role":     "node",
	}
	nodeSelectors := map[string]string{
		"blah":               "blerp",
		"kubernetes.io/role": "node",
	}
	//nodeSelectors := map[string]string{}
	expectedResults := []bool{
		true,
		false,
	}
	test.Log("testing nodelabelsmatch")
	r := nodeLabelsMatch(labels, nodeSelectors)
	if r == false {
		test.Errorf("Expected %v but got: %v", expectedResults[0], r)
	}
}
