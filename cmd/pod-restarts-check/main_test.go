package main

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func pod(podName string, containerName string, restartCount int32) *v1.Pod {

	var pod = v1.Pod{
		ObjectMeta: v12.ObjectMeta{
			Namespace: "test-namespace",
			Name:      podName,
		},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Reason: "Ready",
			ContainerStatuses: []v1.ContainerStatus{
				{Name: containerName,
					Ready:        true,
					RestartCount: restartCount},
			},
		},
	}

	return &pod
}

func makeRestartObservationsMap(podName string, containerName string, restartCount int32) map[string]map[string]int32 {
	firstMap := make(map[string]int32)
	firstMap[containerName] = restartCount

	restartObservationsMap := make(map[string]map[string]int32)
	restartObservationsMap[podName] = firstMap

	return restartObservationsMap
}

func TestAddPodRestartCount(t *testing.T) {

	expectedRestartObservations := makeRestartObservationsMap("test-pod", "container-name", 5)

	var testCase = struct {
		description string
		pod         v1.Pod
		expected    map[string]map[string]int32
	}{"Pod: test-pod, container: container-name, restart count: 5",
		*pod("test-pod", "container-name", 5),
		expectedRestartObservations}

	prc := New()

	prc.addPodRestartCount(testCase.pod)
	if !reflect.DeepEqual(prc.RestartObservations, testCase.expected) {
		t.Fatalf("AddPodRestartCount resulted in %v but expected value %v", prc.RestartObservations, testCase.expected)
	}
	t.Logf("AddPodRestartCount resulted in %v correctly", prc.RestartObservations)
}

func TestCheckBadPodRestarts(t *testing.T) {
	prc := New()
	prc.RestartObservations = makeRestartObservationsMap("bad-test-pod", "bad-test-container", 3)

	goodPodOld := make(map[string]int32)
	goodPodOld["good-test-container"] = 2
	prc.RestartObservations["good-test-pod"] = goodPodOld

	var testCases = []struct {
		description string
		pod         v1.Pod
	}{
		{
			"Bad pod with more than max restarts",
			*pod("bad-test-pod", "bad-test-container", 10),
		},
		{
			"Good pod with less than max restarts",
			*pod("good-test-pod", "good-test-container", 3),
		},
	}

	for _, test := range testCases {
		prc.checkBadPodRestarts(test.pod)
		_, exists := BadPodRestarts.Load(test.pod.Name)
		switch test.pod.Name {
		case "bad-test-pod":
			if !exists {
				t.Fatalf("CheckBadPodRestarts did not store bad pod: %s in BadPodRestarts", test.pod.Name)
			}
			t.Logf("CheckBadPodRestarts stored pod: %s in BadPodRestarts correctly", test.pod.Name)
		case "good-test-pod":
			if exists {
				t.Fatalf("CheckBadPodRestarts stored good pod: %s in BadPodRestarts incorrectly", test.pod.Name)
			}
			t.Logf("CheckBadPodRestarts did not store good pod: %s in BadPodRestarts correctly", test.pod.Name)
		}
	}

}

func TestRemoveBadPodRestarts(t *testing.T) {

	prc := New()
	BadPodRestarts.Store("removed-bad-pod", 3)
	BadPodRestarts.Store("bad-test-pod", 8)

	var testCases = []struct {
		description string
		pod         v1.Pod
	}{
		{"Remove deleted pod from BadPodRestarts", *pod("removed-bad-pod", "removed-bad-container", 3)},
		{"Don't remove pod from BadPodRestarts", *pod("bad-test-pod", "bad-test-container", 8)},
	}

	prc.removeBadPodRestarts("removed-bad-pod")
	for _, test := range testCases {
		_, exists := BadPodRestarts.Load(test.pod.Name)
		switch test.pod.Name {
		case "removed-bad-pod":
			if exists {
				t.Fatalf("RemoveBadPodRestarts failed to remove deleted pod: %s in BadPodRestarts", test.pod.Name)
			}
			t.Logf("RemoveBadPodRestarts correctly removed deleted pod: %s in BadPodRestarts", test.pod.Name)
		case "bad-test-pod":
			if !exists {
				t.Fatalf("RemoveBadPodRestarts incorrectly removed bad pod: %s in BadPodRestarts", test.pod.Name)
			}
			t.Logf("RemoveBadPodRestarts correctly contains bad pod: %s in BadPodRestarts", test.pod.Name)
		}
	}

}
