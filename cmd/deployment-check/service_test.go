package main

import (
	"testing"
)

func TestCreateServiceConfig(t *testing.T) {
	cases := []string{"test-image:latest", "nginx:latest", "nginx:test"}
	for _, c := range cases {
		deploymentConfig := createDeploymentConfig(c)

		if deploymentConfig.Spec.Selector == nil {
			t.Fatalf("deployment config was created without selctors: %v\n", deploymentConfig.Spec.Selector)
		}

		if len(deploymentConfig.Spec.Selector.MatchLabels) == 0 {
			t.Fatalf("deployment config was created without selctor match labels: %v\n", deploymentConfig.Spec.Selector)
		}

		serviceConfig := createServiceConfig(deploymentConfig.Spec.Selector.MatchLabels)

		if len(serviceConfig.Name) == 0 {
			t.Fatalf("nil service name: %s\n", serviceConfig.Name)
		}

		if len(serviceConfig.Name) == 0 {
			t.Fatalf("nil service namespace: %s\n", serviceConfig.Name)
		}

		if len(serviceConfig.Spec.Ports) == 0 {
			t.Fatalf("no ports created for service: %d ports", len(serviceConfig.Spec.Ports))
		}
	}
}
