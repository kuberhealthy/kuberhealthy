package external_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	// "k8s.io/apimachinery/pkg/api/resource"

	log "github.com/sirupsen/logrus"

	"github.com/Comcast/kuberhealthy/pkg/checks/external"
	"github.com/Comcast/kuberhealthy/pkg/kubeClient"

	apiv1 "k8s.io/api/core/v1"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// kubeConfigFile is the config file location for kubernetes
var kubeConfigFile =  os.Getenv("HOME") + "/.kube/config"


func init(){
	// tests always run with debug logging
	log.SetLevel(log.DebugLevel)
}

// loadTestPodSpecFile loads a pod spec yaml from disk in this
// directory and returns the pod spec struct it represents
func loadTestPodSpecFile(path string) (*apiv1.PodSpec, error) {

	podSpec := apiv1.PodSpec{}

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
		t.Log(err)
		t.Fail()
		return
	}

	t.Log(string(b))
}

// TestLoadPodSpecTestFile test loads a test pod spec from a yaml file into a PodSpec struct
func TestLoadPodSpecTestFile(t *testing.T) {
	t.Parallel()
	p, err := loadTestPodSpecFile("test/basicCheckerPod.yaml")
	if err != nil {
		t.Log("Error loading test file:", err)
		t.FailNow()
		return
	}
	t.Log(p)
}

// TestExternalCheckerSanitation tests the external checker in a situation
// where it should fail
func TestExternalCheckerSanitation(t *testing.T) {
	t.Parallel()

	// create a kubernetes clientset
	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		t.Log("Unable to create kubernetes client", err)
	}
	// make a new default checker of this check
	checker, err := newTestCheck()
	if err != nil {
		t.Log("Failed to create client:",err)
	}
	checker.KubeClient = client

	// sabotage the pod name
	checker.PodName = ""

	// run the checker with the kube client
	err = checker.RunOnce()
	if err == nil {
		t.Log("Expected pod name blank validation check failure but did not hit it")
		t.FailNow()
	}
	t.Log("got expected error:",err)

	// break the pod namespace instead now
	checker.PodName = external.DefaultName
	checker.Namespace = ""

	// run the checker with the kube client
	err = checker.RunOnce()
	if err == nil {
		t.Log("Expected namespace blank validation check failure but did not hit it")
		t.FailNow()
	}
	t.Log("got expected error:",err)

	// break the pod spec now instead of namespace
	checker.Namespace = "kuberhealthy"
	checker.PodSpec = &apiv1.PodSpec{}

	// run the checker with the kube client
	err = checker.RunOnce()
	if err == nil {
		t.Log("Expected pod validation check failure but did not hit it")
		t.FailNow()
	}
	t.Log("got expected error:",err)
}

// TestExternalChecker tests the external checker end to end
func TestExternalChecker(t *testing.T) {

	// create a kubernetes clientset
	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		t.Log("Unable to create kubernetes client", err)
	}

	// make a new default checker of this check
	checker, err := newTestCheck()
	if err != nil {
		t.Log("Failed to create client:",err)
	}
	checker.KubeClient = client

	// run the checker with the kube client
	err = checker.RunOnce()
	if err != nil {
		t.Fatal(err)
	}
}

// TestShutdown tests shutting down a check while its running
func TestShutdown(t *testing.T) {
	// create a kubernetes clientset
	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		t.Log("Unable to create kubernetes client", err)
	}

	// make a new default checker of this check
	checker, err := newTestCheck()
	if err != nil {
		t.Log("Failed to create client:",err)
	}
	checker.KubeClient = client

	// run the checker with the kube client
	t.Log("Starting check...")
	go func(){
		err := checker.RunOnce()
		if err != nil {
			t.Log("Failure when running check:",err)
			t.FailNow()
		}
	}()

	// give the check a few seconds to start
	t.Log("Waiting for check to get started...")
	time.Sleep(time.Second * 10)

	// tell the checker to shut down in the background
	t.Log("Sending shutdown to check")
	c := make(chan error,0)
	go func(c chan error){
		c<- checker.Shutdown()
	}(c)

	// see if we shut down properly before a timeout
	select {
	case <-time.After(time.Second * 20):
		t.Log("Failed to interrupt and shut down pod properly")
		t.FailNow()
		case e := <-c:
			// see if the check shut down without error
			if e != nil {
				t.Log("Error shutting down in-flight check:",err)
				t.FailNow()
			}
			t.Log("Check shutdown properly and without error")
	}


}


// newTestCheck creates a new test checker struct with a basic set of defaults
// that work out of the box
func newTestCheck() (*external.Checker, error) {
	podCheckFile := "test/basicCheckerPod.yaml"
	p, err := loadTestPodSpecFile(podCheckFile)
	if err != nil {
		return &external.Checker{}, errors.New("Unable to load kubernetes pod spec " + podCheckFile + " " + err.Error())
	}
	return newTestCheckFromSpec(p), nil
}

// newTestCheckFromSpec creates a new test checker but using the supplied
// spec file for pods
func newTestCheckFromSpec(spec *apiv1.PodSpec) *external.Checker {
	// create a new checker and insert this pod spec
	checker := external.New(spec) // external checker does not ever return an error so we drop it
	checker.Namespace = "kuberhealthy"
	checker.Debug = true
	return checker
}
