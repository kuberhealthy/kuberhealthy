package main

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func pod(podName string, containerName string, restartCount int32) *v1.Pod {

	var pod = v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
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
