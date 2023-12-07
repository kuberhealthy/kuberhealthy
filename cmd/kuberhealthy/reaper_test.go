package main

import (
	"context"
	"strings"
	"testing"
	"time"

	yaml "gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestParseConfigs ensures that all checkReaper configs are properly parsed and that there are no 0 duration values
func TestParseConfigs(t *testing.T) {

	inputConfig := `listenAddress: ":8080" # The port for kuberhealthy to listen on for web requests
enableForceMaster: false # Set to true to enable local testing, forced master mode
logLevel: "debug" # Log level to be used
listenNamespace: "test" # Kuberhealthy will only monitor khcheck resources from this namespace
jobCleanupDuration: 3m # The maximum age of khjobs before being reaped
maxCheckPods: 1 # The maximum number of check pods in Completed state before being reaped`

	testConfig := Config{}
	err := yaml.Unmarshal([]byte(inputConfig), &testConfig)
	if err != nil {
		t.Fatal("Error unmarshaling yaml config with error:" + err.Error())
	}

	if testConfig.ListenAddress != ":8080" {
		t.Fatal("ListenAddress did not unmarshal correctly")
	}

	if testConfig.LogLevel != "debug" {
		t.Fatal("ListenAddress did not unmarshal correctly")
	}

}

// TestParseStringDuration ensures that a string duration can be parsed into a time.Duration.
// If string is invalid or empty, return the defaultDuration.
// If the parsed time.Duration is 0, return defaultDuration.
func TestParseDurationOrUseDefault(t *testing.T) {

	var testCases = []struct {
		description     string
		stringDuration  string
		defaultDuration time.Duration
		expected        time.Duration
		err             string
	}{
		{"Valid duration", "5m", time.Minute * 15, time.Minute * 5, ""},
		{"0 Duration value, not allowed", "0", time.Minute * 15, time.Minute * 15, "duration value 0 is not valid"},
		{"No duration value", "", time.Minute * 15, time.Minute * 15, ""},
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
func TestListCompletedCheckerPods(t *testing.T) {
	validKHPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
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
		ObjectMeta: metav1.ObjectMeta{
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      "non-kh-pod",
			Namespace: "foo",
		},
	}

	runningKHPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
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

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()
	for _, c := range khCheckerPods {
		_, err := api.Client.CoreV1().Pods(c.Namespace).Create(ctx, &c, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Error creating test pods: %s", err)
		}
	}

	results, err := api.listCompletedCheckerPods(ctx, "")
	if err != nil {
		t.Fatalf("Error listCompletedCheckerPods: %s", err)
	}

	if len(results) != 2 {
		t.Fatalf("listCompletedCheckerPods failed to list only completed Kuberhealthy pods")
	}

	if _, exists := results[validKHPod.Name]; !exists {
		t.Fatalf("listCompletedCheckerPods failed to list %s", validKHPod.Name)
	}

	if _, exists := results[anotherValidKHPod.Name]; !exists {
		t.Fatalf("listCompletedCheckerPods failed to list %s", anotherValidKHPod.Name)
	}

	t.Logf("listCompletedCheckerPods successfully listed only completed Kuberhealthy pods")
}

// TestGetAllPodsWithCheckName tests that only pods from the same khcheck get listed
func TestGetAllPodsWithCheckName(t *testing.T) {

	khCheck := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
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
		ObjectMeta: metav1.ObjectMeta{
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
		ObjectMeta: metav1.ObjectMeta{
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
		ObjectMeta: metav1.ObjectMeta{
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

	results := getAllCompletedPodsWithCheckName(khCheckerPods, khCheck)

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
