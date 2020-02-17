package external

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	// "k8s.io/apimachinery/pkg/api/resource"

	log "github.com/sirupsen/logrus"

	"github.com/Comcast/kuberhealthy/v2/pkg/khcheckcrd"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"

	apiv1 "k8s.io/api/core/v1"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func init() {
	// tests always run with debug logging
	log.SetLevel(log.DebugLevel)
}

// loadTestPodSpecFile loads a check spec yaml from disk in this
// the test directory and returns the check struct
func loadTestPodSpecFile(path string) (*khcheckcrd.KuberhealthyCheck, error) {

	podSpec := khcheckcrd.KuberhealthyCheck{}

	// open the yaml file
	f, err := os.Open(path)
	if err != nil {
		return &podSpec, err
	}

	// read in all the configuration bytes
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return &podSpec, err
	}

	log.Debugln("Decoding this YAML:", string(b))
	j, err := yaml.YAMLToJSON(b)

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

// TestShutdown tests shutting down a check while its running
func TestShutdown(t *testing.T) {
	// create a kubernetes clientset
	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		t.Log("Unable to create kubernetes client", err)
	}

	// make a new default checker of this check
	checker, err := newTestChecker(client)
	if err != nil {
		t.Log("Failed to create client:", err)
	}
	checker.KubeClient = client

	// run the checker with the kube client
	t.Log("Starting check...")
	go func() {
		err := checker.RunOnce()
		if err != nil {
			t.Fatal("Failure when running check:", err)
		}
	}()

	// give the check a few seconds to start
	t.Log("Waiting for check to get started...")
	time.Sleep(time.Second * 20)

	// tell the checker to shut down in the background
	t.Log("Sending shutdown to check")
	c := make(chan error, 0)
	go func(c chan error) {
		c <- checker.Shutdown()
	}(c)

	// see if we shut down properly before a timeout
	select {
	case <-time.After(time.Second * 20):
		t.Log("Failed to interrupt and shut down pod properly")
		t.FailNow()
	case e := <-c:
		// see if the check shut down without error
		if e != nil {
			t.Fatal("Error shutting down in-flight check:", err)
		}
		t.Log("Check shutdown properly and without error")
	}
}
