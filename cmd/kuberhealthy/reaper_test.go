package main

import (
	"context"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestParseConfigs ensures that all checkReaper configs are properly parsed and that there are no 0 values
func TestParseConfigs(t *testing.T) {

	testConfig1 := &Config{
		FailedPodCleanupDuration: "10m",
		JobCleanupDuration: "100h",
		MaxCheckPods: 4,
	}

	testConfig2 := &Config{
		FailedPodCleanupDuration: "10m",
		JobCleanupDuration: "100h",
		MaxCheckPods: 0,
	}

	testConfig2.parseConfigs()

	var testCases = []struct {
		description 	string
		cfg         	*Config
	}{
		{"Valid config", testConfig1},
		{"0 MaxCheckPods", testConfig2},
	}

	for _, test := range testCases {

		t.Logf(test.description)

		test.cfg.parseConfigs()

		if test.cfg.failedPodCleanupDuration == 0 {
			t.Fatalf("failedPodCleanupDuration config not set")
		}
		t.Logf("parseConfigs parsed failedPodCleanupDuration correctly: %s", test.cfg.failedPodCleanupDuration)

		if test.cfg.jobCleanupDuration == 0 {
			t.Fatalf("jobCleanupDuration config not set")
		}
		t.Logf("parseConfigs parsed jobCleanupDuration correctly: %s", test.cfg.jobCleanupDuration)

		if test.cfg.MaxCheckPods == 0 {
			t.Fatalf("MaxCheckPods config not set")
		}
		t.Logf("parseConfigs parsed MaxCheckPods correctly: %d", test.cfg.MaxCheckPods)

	}

}

// TestParseStringDuration ensures that a string duration can be parsed into a time.Duration.
// If string is invalid or empty, return the defaultDuration.
// If the parsed time.Duration is 0, return defaultDuration.
func TestParseDurationOrUseDefault(t *testing.T) {

	var testCases = []struct {
		description 	string
		stringDuration	string
		defaultDuration	time.Duration
		expected    	time.Duration
		err				string
	}{
		{"Valid duration", "5m", time.Minute*15, time.Minute*5, ""},
		{"Invalid Duration, failure to parse", "invalid", time.Minute*15, time.Minute*15, "time: invalid duration \"invalid\""},
		{"0 Duration value, not allowed", "0", time.Minute*15, time.Minute*15, ""},
		{"No duration value", "", time.Minute*15, time.Minute*15, ""},
	}

	for _, test := range testCases {

		t.Logf(test.description)

		result, err := parseDurationOrUseDefault(test.stringDuration, test.defaultDuration)
		if result != test.expected {
			t.Fatalf("parseDurationOrUseDefault resulted in %s but expected value %s", result, test.expected)
		}
		if err != nil {
			if err.Error() != test.err {
				t.Fatalf("parseDurationOrUseDefault err is %s but expected err %s", err.Error(), test.err)
			}
		}
		t.Logf("parseStringDuration resulted in %s correctly", result)
	}
}

// TestListCheckerPods ensures that only completed (Successful or Failed) kuberhealthy checker pods are listed / returned
func TestListCheckerPods(t *testing.T) {
	validKHPod := v1.Pod{
		ObjectMeta: v12.ObjectMeta{
			Name:      "valid-kh-pod",
			Namespace: "foo",
			Labels: map[string]string{
				"kuberhealthy-check-name": "valid-kh-pod",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPhase("Succeeded"),
		},
	}

	anotherValidKHPod := v1.Pod{
		ObjectMeta: v12.ObjectMeta{
			Name:      "another-valid-kh-pod",
			Namespace: "foo",
			Labels: map[string]string{
				"kuberhealthy-check-name": "another-valid-kh-pod",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPhase("Failed"),
		},
	}

	nonKHPod := v1.Pod{
		ObjectMeta: v12.ObjectMeta{
			Name:      "non-kh-pod",
			Namespace: "foo",
		},
	}

	runningKHPod := v1.Pod{
		ObjectMeta: v12.ObjectMeta{
			Name:      "running-kh-pod",
			Namespace: "foo",
			Labels: map[string]string{
				"kuberhealthy-check-name": "running-kh-pod",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPhase("Running"),
		},
	}

	khCheckerPods := make(map[string]v1.Pod)
	khCheckerPods[validKHPod.Name] = validKHPod
	khCheckerPods[anotherValidKHPod.Name] = anotherValidKHPod
	khCheckerPods[nonKHPod.Name] = nonKHPod
	khCheckerPods[runningKHPod.Name] = runningKHPod

	api := KubernetesAPI{
		Client: fake.NewSimpleClientset(),
	}

	ctx, _ := context.WithCancel(context.Background())
	for _, c := range khCheckerPods {
		_, err := api.Client.CoreV1().Pods(c.Namespace).Create(ctx, &c, v12.CreateOptions{})
		if err != nil {
			t.Fatalf("Error creating test pods: %s", err)
		}
	}

	results, err := api.listCheckerPods(ctx, "")
	if err != nil {
		t.Fatalf("Error listCheckerPods: %s", err)
	}

	if len(results) != 2 {
		t.Fatalf("listCheckerPods failed to list only completed Kuberhealthy pods")
	}

	if _, exists := results[validKHPod.Name]; !exists {
		t.Fatalf("listCheckerPods failed to list %s", validKHPod.Name)
	}

	if _, exists := results[anotherValidKHPod.Name]; !exists {
		t.Fatalf("listCheckerPods failed to list %s", anotherValidKHPod.Name)
	}

	t.Logf("listCheckerPods successfully listed only completed Kuberhealthy pods")
}

// TestGetAllPodsWithCheckName tests that only pods from the same khcheck get listed
func TestGetAllPodsWithCheckName(t *testing.T) {

	khCheck := v1.Pod{
		ObjectMeta: v12.ObjectMeta{
			Name:      "original-kh-pod",
			Namespace: "foo",
			Annotations: map[string]string{
				"comcast.github.io/check-name": "same-check",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPhase("Succeeded"),
		},
	}

	khCheck1 := v1.Pod{
		ObjectMeta: v12.ObjectMeta{
			Name:      "same-kh-pod",
			Namespace: "foo",
			Labels: map[string]string{
				"kuberhealthy-check-name": "same-check",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPhase("Succeeded"),
		},
	}

	khCheck2 := v1.Pod{
		ObjectMeta: v12.ObjectMeta{
			Name:      "another-same-kh-pod",
			Namespace: "foo",
			Labels: map[string]string{
				"kuberhealthy-check-name": "same-check",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPhase("Succeeded"),
		},
	}

	khCheck3 := v1.Pod{
		ObjectMeta: v12.ObjectMeta{
			Name:      "different-kh-pod",
			Namespace: "foo",
			Labels: map[string]string{
				"kuberhealthy-check-name": "different-check",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPhase("Succeeded"),
		},
	}

	khCheckerPods := make(map[string]v1.Pod)
	khCheckerPods[khCheck1.Name] = khCheck1
	khCheckerPods[khCheck2.Name] = khCheck2
	khCheckerPods[khCheck3.Name] = khCheck3

	results := getAllPodsWithCheckName(khCheckerPods, khCheck)

	if len(results) != 2 {
		t.Fatalf("getAllPodsWithCheckName failed to get all pods of the same khcheck")
	}

	for _, pod := range results {
		if !strings.Contains(pod.Name, "same-kh-pod") {
			t.Fatalf("getAllPodsWithCheckName got a pod not from the same khcheck: %s", pod.Name)
		}
	}

	t.Logf("getAllPodsWithCheckName successfully listed all pods from the same khcheck")
}

//TODO: TestDeleteFilteredCheckerPods
