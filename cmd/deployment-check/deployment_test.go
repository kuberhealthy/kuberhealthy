package main

import (
	"testing"
)

func TestCreateContainerConfig(t *testing.T) {
	cases := []string{"test-image:latest", "nginx:latest", "nginx:test"}
	for _, c := range cases {
		containerConfig := createContainerConfig(c)

		if len(containerConfig.Name) == 0 {
			t.Fatalf("nil container name: %s\n", containerConfig.Name)
		}

		if len(containerConfig.Image) == 0 {
			t.Fatalf("nil container image: %s\n", containerConfig.Image)
		}

		if containerConfig.Image != c {
			t.Fatalf("expected container image to be %s but got: %s\n", c, containerConfig.Image)
		}

		if len(containerConfig.ImagePullPolicy) == 0 {
			t.Fatalf("nil image pull policy: %s", containerConfig.ImagePullPolicy)
		}

		if len(containerConfig.Ports) == 0 {
			t.Fatalf("no ports given for container: found %d ports\n", len(containerConfig.Ports))
		}

		if containerConfig.LivenessProbe == nil {
			t.Fatalf("nil container liveness probe: %v\n", containerConfig.LivenessProbe)
		}

		if containerConfig.ReadinessProbe == nil {
			t.Fatalf("nil container readiness probe: %v\n", containerConfig.ReadinessProbe)
		}
	}
}

func TestCreateDeploymentConfig(t *testing.T) {
	cases := []string{"test-image:latest", "nginx:latest", "nginx:test"}
	for _, c := range cases {
		deploymentConfig := createDeploymentConfig(c)

		if len(deploymentConfig.ObjectMeta.Name) == 0 {
			t.Fatalf("nil deployment object meta name: %s\n", deploymentConfig.ObjectMeta.Name)
		}

		if len(deploymentConfig.ObjectMeta.Namespace) == 0 {
			t.Fatalf("nil deployment object meta namespace: %s\n", deploymentConfig.ObjectMeta.Namespace)
		}

		reps := int32(1)
		if *deploymentConfig.Spec.Replicas < reps {
			t.Fatalf("deployment config was created with less than 1 replica: %d", deploymentConfig.Spec.Replicas)
		}

		if deploymentConfig.Spec.Selector == nil {
			t.Fatalf("deployment config was created without selctors: %v", deploymentConfig.Spec.Selector)
		}

		if len(deploymentConfig.Spec.Selector.MatchLabels) == 0 {
			t.Fatalf("deployment config was created without selctor match labels: %v", deploymentConfig.Spec.Selector)
		}
	}
}
