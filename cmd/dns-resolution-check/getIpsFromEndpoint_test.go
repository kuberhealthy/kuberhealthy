package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kuberhealthy/kuberhealthy/v2/pkg/kubeClient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetIpsFromEndpoint(t *testing.T) {
	client, err := kubeClient.Create(filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	if err != nil {
		t.Fatalf("Unable to create kube client")
	}
	endpoints, err := client.CoreV1().Endpoints(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		t.Fatalf("Unable to get endpoint list %+v\n", err)
	}

	ips, err := getIpsFromEndpoint(endpoints)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if len(ips) < 1 {
		t.Fatalf("No ips found from endpoint list")
	}

}
