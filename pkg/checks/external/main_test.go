package external_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/ghodss/yaml"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	// "k8s.io/apimachinery/pkg/api/resource"

	"github.com/Comcast/kuberhealthy/pkg/checks/external"
	"github.com/Comcast/kuberhealthy/pkg/kubeClient"
	log "github.com/sirupsen/logrus"

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

	fmt.Println("Decoding this YAML:")
	fmt.Println(string(b))
	j, err := yaml.YAMLToJSON(b)


	// unmarshal the pod into the pod struct and return
	err = json.Unmarshal(j, &podSpec)
	return &podSpec, err
}

// TestOutputPodSpecAsYAML outputs YAML for a pod spec and verifies it can be marshaled
func TestOutputPodSpecAsYAML(t *testing.T) {
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
		p, err := loadTestPodSpecFile("test/basicCheckerPod.yaml")
		if err != nil {
			t.Log("Error loading test file:", err)
			t.FailNow()
			return
		}
		t.Log(p)
}

// TestExternalChecker tests the external checker end to end
func TestExternalChecker(t *testing.T) {

	// create a kubernetes clientset
	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		log.Errorln("Unable to create kubernetes client", err)
	}

	// load the test pod spec file into a pod spec
	podCheckFile := "test/basicCheckerPod.yaml"
	p, err := loadTestPodSpecFile(podCheckFile)
	if err != nil {
		log.Errorln("Unable to load kubernetes pod spec " + podCheckFile, err)
	}

	// make a new default checker of this check
	checker := newTestCheck(p)

	// run the checker with the kube client
	err = checker.Run(client)
	if err != nil {
		t.Fatal(err)
	}
}


// newTestCheck creates a new test checker struct with
// a basic set of defaults that work
func newTestCheck(spec *apiv1.PodSpec) *external.Checker {
	// create a new checker and insert this pod spec
	checker, _ := external.New() // external checker does not ever return an error so we drop it
	checker.PodSpec = spec
	checker.Namespace = "kuberhealthy"
	checker.Debug = true
	return checker
}
