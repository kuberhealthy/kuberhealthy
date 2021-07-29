package masterCalculation

import (
	"os"
	"testing"

	"github.com/kuberhealthy/kuberhealthy/v2/pkg/kubeClient"
	log "github.com/sirupsen/logrus"
)

var kubeConfigFile = os.Getenv("HOME") + "/.kube/config"

func TestRun(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		t.Fatal(err)
	}

	master, err := CalculateMaster(client)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(master)
}
