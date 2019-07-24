package external

import (
	"errors"
	"testing"

	"github.com/Comcast/kuberhealthy/pkg/kubeClient"

	apiv1 "k8s.io/api/core/v1"
)

// newTestCheck creates a new test checker struct with a basic set of defaults
// that work out of the box
func newTestCheck() (*Checker, error) {
	podCheckFile := "test/basicCheckerPod.yaml"
	p, err := loadTestPodSpecFile(podCheckFile)
	if err != nil {
		return &Checker{}, errors.New("Unable to load kubernetes pod spec " + podCheckFile + " " + err.Error())
	}
	return newTestCheckFromSpec(p), nil
}

// TestExternalChecker tests the external checker end to end
func TestExternalChecker(t *testing.T) {

	// create a kubernetes clientset
	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		t.Fatal("Unable to create kubernetes client", err)
	}

	// make a new default checker of this check
	checker, err := newTestCheck()
	if err != nil {
		t.Fatal("Failed to create client:",err)
	}
	checker.KubeClient = client

	// run the checker with the kube client
	err = checker.RunOnce()
	if err != nil {
		t.Fatal(err)
	}
}

// newTestCheckFromSpec creates a new test checker but using the supplied
// spec file for pods
func newTestCheckFromSpec(spec *apiv1.PodSpec) *Checker {
	// create a new checker and insert this pod spec
	checker := New(spec) // external checker does not ever return an error so we drop it
	checker.Namespace = defaultNamespace
	checker.Debug = true
	return checker
}

// TestExternalCheckerSanitation tests the external checker in a situation
// where it should fail
func TestExternalCheckerSanitation(t *testing.T) {
	t.Parallel()

	// create a kubernetes clientset
	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		t.Fatal("Unable to create kubernetes client", err)
	}
	// make a new default checker of this check
	checker, err := newTestCheck()
	if err != nil {
		t.Fatal("Failed to create client:",err)
	}
	checker.KubeClient = client

	// sabotage the pod name
	checker.PodName = ""

	// run the checker with the kube client
	err = checker.RunOnce()
	if err == nil {
		t.Fatal("Expected pod name blank validation check failure but did not hit it")
	}
	t.Log("got expected error:",err)

	// break the pod namespace instead now
	checker.PodName = DefaultName
	checker.Namespace = ""

	// run the checker with the kube client
	err = checker.RunOnce()
	if err == nil {
		t.Fatal("Expected namespace blank validation check failure but did not hit it")
	}
	t.Log("got expected error:",err)

	// break the pod spec now instead of namespace
	checker.Namespace = defaultNamespace
	checker.PodSpec = &apiv1.PodSpec{}

	// run the checker with the kube client
	err = checker.RunOnce()
	if err == nil {
		t.Fatal("Expected pod validation check failure but did not hit it")
	}
	t.Log("got expected error:",err)
}

// TestGetWhitelistedUUIDForExternalCheck validates that setting
// and fetching whitelist UUIDs works properly
func TestGetWhitelistedUUIDForExternalCheck(t *testing.T) {
	var testUUID = "test-UUID-1234"

	// make an external check and cause it to write a whitelist
	c, err := newTestCheck()
	if err != nil {
		t.Fatal("Failed to create new external check:",err)
	}
	c.KubeClient, err = kubeClient.Create(kubeConfigFile)
	if err != nil {
		t.Fatal("Failed to create kube client:",err)
	}

	// delete the UUID (blank it out)
	err = c.setUUID("")
	if err != nil {
		t.Fatal("Failed to blank the UUID on test check:", err)
	}

	// get the UUID's value (should be blank)
	uuid, err := GetWhitelistedUUIDForExternalCheck(c.Name())
	if err != nil {
		t.Fatal("Failed to get UUID on test check:", err)
	}
	if uuid != "" {
		t.Fatal("Found a UUID set for a check when we expected to see none:", err)
	}

	// set the UUID for real this time
	err = c.setUUID(testUUID)
	if err != nil {
		t.Fatal("Failed to set UUID on test check:", err)
	}

	// re-fetch the UUID from the custom resource
	uuid, err = GetWhitelistedUUIDForExternalCheck(c.Name())
	if err != nil {
		t.Fatal("Failed to get UUID on test check:", err)
	}
	t.Log("Fetched UUID :", uuid)

	if uuid != testUUID {
		t.Fatal("Tried to set and fetch a UUID on a check but the UUID did not match what was set.")
	}
}

func TestSanityCheck(t *testing.T) {
	c, err := newTestCheck()
	if err != nil {
		t.Fatal(err)
	}

	// by default with the test checker, the sanity test should pass
	err = c.sanityCheck()
	if err != nil {
		t.Fatal(err)
	}

	// if we blank the namespace, it should fail
	c.Namespace = ""
	err = c.sanityCheck()
	if err == nil {
		t.Fatal(err)
	}

	// fix the namespace, then try blanking the PodName
	c.Namespace = defaultNamespace
	c.PodName = ""
	err = c.sanityCheck()
	if err == nil {
		t.Fatal(err)
	}

	// fix the pod name and try KubeClient
	c.PodName = "kuberhealthy"
	c.KubeClient = nil
	err = c.sanityCheck()
	if err == nil {
		t.Fatal(err)
	}

}
