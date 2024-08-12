package external

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/ghodss/yaml"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	log "github.com/sirupsen/logrus"

	apiv1 "k8s.io/api/core/v1"

	khcrds "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/comcast.github.io/v1"
)

func init() {
	// tests always run with debug logging
	log.SetLevel(log.DebugLevel)
}

// loadTestPodSpecFile loads a check spec yaml from disk in this
// the test directory and returns the check struct
func loadTestPodSpecFile(path string) (*khcrds.KuberhealthyCheck, error) {

	podSpec := khcrds.KuberhealthyCheck{}

	// read in all the configuration bytes
	b, err := os.ReadFile(path)
	if err != nil {
		return &podSpec, err
	}

	log.Debugln("Decoding this YAML:", string(b))
	j, err := yaml.YAMLToJSON(b)
	if err != nil {
		return &podSpec, err
	}

	// unmarshal the pod into the pod struct and return
	err = json.Unmarshal(j, &podSpec)
	return &podSpec, err
}

// TestOutputPodSpecAsYAML outputs YAML for a pod spec and verifies it can be marshaled
func TestOutputPodSpecAsYAML(t *testing.T) {
	t.Parallel()
	p := apiv1.PodSpec{}
	b, err := yaml.Marshal(p)
	if err != nil {
		t.Fatal(err)
		return
	}

	t.Log(string(b))
}

// TestLoadPodSpecTestFile test loads a test pod spec from a yaml file into a PodSpec struct
func TestLoadPodSpecTestFile(t *testing.T) {
	t.Parallel()
	p, err := loadTestPodSpecFile("test/basicCheckerPod.yaml")
	if err != nil {
		t.Fatal("Error loading test file:", err)
		return
	}
	t.Log(p)
}

// // TestShutdown tests shutting down a check while its running
// func TestShutdown(t *testing.T) {
// 	// create a kubernetes clientset
// 	client, _, err := kubeClient.Create(kubeConfigFile)
// 	if err != nil {
// 		t.Log("Unable to create kubernetes client", err)
// 	}

// 	// make a new default checker of this check
// 	checker, err := newTestChecker()
// 	if err != nil {
// 		t.Log("Failed to create client:", err)
// 	}
// 	checker.KubeClientInterface = client

// 	// run the checker with the kube client
// 	t.Log("Starting check...")
// 	go func(t *testing.T) {
// 		err := checker.RunOnce(context.Background())
// 		if err != nil {
// 			t.Error("Failure when running check:", err)
// 		}
// 	}(t)

// 	// give the check a few seconds to start
// 	t.Log("Waiting for check to get started...")
// 	time.Sleep(time.Second * 20)

// 	// tell the checker to shut down in the background
// 	t.Log("Sending shutdown to check")
// 	c := make(chan error)
// 	go func(c chan error) {
// 		c <- checker.Shutdown()
// 	}(c)

// 	// see if we shut down properly before a timeout
// 	select {
// 	case <-time.After(time.Second * 20):
// 		t.Log("Failed to interrupt and shut down pod properly")
// 		t.FailNow()
// 	case e := <-c:
// 		// see if the check shut down without error
// 		if e != nil {
// 			t.Fatal("Error shutting down in-flight check:", err)
// 		}
// 		t.Log("Check shutdown properly and without error")
// 	}
// }
