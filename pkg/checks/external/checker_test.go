package external

import (
	"context"
	"errors"
	"log"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	khcheckv1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khcheck/v1"
	khstatev1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khstate/v1"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/kubeClient"

	apiv1 "k8s.io/api/core/v1"
)

var client *kubernetes.Clientset

const defaultNamespace = "kuberhealthy"

var khStateClient *khstatev1.KHStateV1Client
var khCheckClient *khcheckv1.KHCheckV1Client

func init() {

	// create a kubernetes clientset for our tests to use
	var err error
	client, err = kubeClient.Create(kubeConfigFile)
	if err != nil {
		log.Fatal("Unable to create kubernetes client", err)
	}

	// make a new crd check client
	checkClient, err := khcheckv1.Client(kubeConfigFile)
	if err != nil {
		log.Fatalln("err")
	}
	khCheckClient = checkClient

	// make a new crd state client
	stateClient, err := khstatev1.Client(kubeConfigFile)
	if err != nil {
		log.Fatalln("err")
	}
	khStateClient = stateClient

}

// newTestChecker creates a new test checker struct with a basic set of defaults
// that work out of the box
func newTestChecker(client *kubernetes.Clientset) (*Checker, error) {
	podCheckFile := "test/basicCheckerPod.yaml"
	p, err := loadTestPodSpecFile(podCheckFile)
	if err != nil {
		return &Checker{}, errors.New("Unable to load kubernetes pod spec " + podCheckFile + " " + err.Error())
	}
	chk := newTestCheckFromSpec(client, p, DefaultKuberhealthyReportingURL)
	return chk, err
}

// TestExternalChecker tests the external checker end to end
func TestExternalChecker(t *testing.T) {

	// make a new default checker of this check
	checker, err := newTestChecker(client)
	if err != nil {
		t.Fatal("Failed to create client:", err)
	}
	checker.KubeClient = client

	// run the checker with the kube client
	err = checker.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

// newTestCheckFromSpec creates a new test checker but using the supplied
// spec file for a khcheck
func newTestCheckFromSpec(client *kubernetes.Clientset, checkSpec *khcheckv1.KuberhealthyCheck, reportingURL string) *Checker {
	// create a new checker and insert this pod spec
	checker := New(client, checkSpec, khCheckClient, khStateClient, reportingURL) // external checker does not ever return an error so we drop it
	checker.Debug = true
	return checker
}

// TestExternalCheckerSanitation tests the external checker in a situation
// where it should fail
func TestExternalCheckerSanitation(t *testing.T) {
	t.Parallel()

	// make a new default checker of this check
	checker, err := newTestChecker(client)
	if err != nil {
		t.Fatal("Failed to create client:", err)
	}
	checker.KubeClient = client

	// sabotage the pod name
	checker.CheckName = ""

	// run the checker with the kube client
	err = checker.RunOnce(context.Background())
	if err == nil {
		t.Fatal("Expected pod name blank validation check failure but did not hit it")
	}
	t.Log("got expected error:", err)

	// break the pod namespace instead now
	checker.CheckName = DefaultName
	checker.Namespace = ""

	// run the checker with the kube client
	err = checker.RunOnce(context.Background())
	if err == nil {
		t.Fatal("Expected namespace blank validation check failure but did not hit it")
	}
	t.Log("got expected error:", err)

	// break the pod spec now instead of namespace
	checker.Namespace = defaultNamespace
	checker.PodSpec = apiv1.PodSpec{}

	// run the checker with the kube client
	err = checker.RunOnce(context.Background())
	if err == nil {
		t.Fatal("Expected pod validation check failure but did not hit it")
	}
	t.Log("got expected error:", err)
}

// createKHCheck writes a khcheck custom resource on the server
func createKHCheckSpec(checkSpec *khcheckv1.KuberhealthyCheck, checkNamespace string) error {

	// make a new crd check client
	checkClient, err := khcheckv1.Client(kubeConfigFile)
	if err != nil {
		return err
	}

	_, err = checkClient.KuberhealthyChecks(checkSpec.Namespace).Create(checkSpec)
	return err
}

// deleteCheckSpec deletes a khcheck object from the server
func deleteKHCheckSpec(checkName string, checkNamespace string) error {

	// make a new crd check client
	checkClient, err := khcheckv1.Client(kubeConfigFile)
	if err != nil {
		return err
	}

	err = checkClient.KuberhealthyChecks(checkNamespace).Delete(checkName, &v1.DeleteOptions{})
	return err
}

// TestWriteWhitelistedUUID tests writing a UUID to a check without
// removing other properties of the check
func TestWriteWhitelistedUUID(t *testing.T) {

	// create a client for kubernetes
	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		t.Fatal("Failed to create kube client:", err)
	}

	// make an external checker for kubernetes
	checker, err := newTestChecker(client)
	if err != nil {
		t.Fatal("Failed to create new external check:", err)
	}

	// generate a fresh UUID for this test
	testUUID := checker.currentCheckUUID
	t.Log("Expecting UUID to be set on check:", testUUID)

	// ensure the check by this name is cleaned off the server
	_ = deleteKHCheckSpec(checker.Name(), checker.CheckNamespace())

	// create a khcheck custom resource using the pod spec from a test khcheck spec file
	s, err := loadTestPodSpecFile("test/basicCheckerPod.yaml")
	if err != nil {
		t.Fatal((err))
	}
	kuberhealthyCheck := khcheckv1.NewKuberhealthyCheck(checker.CheckName, defaultNamespace, s.Spec)

	// write the check to the server
	err = createKHCheckSpec(&kuberhealthyCheck, defaultNamespace)
	if err != nil {
		t.Fatal(err)
	}

	// set the whitelisted UUID on the server custom resource
	err = checker.setUUID(testUUID)
	if err != nil {
		t.Fatal((err))
	}

	// fetch the khcheck custom resource state from the server to validate it now that the right UUID has been set
	c, err := checker.getCheck()
	if err != nil {
		t.Fatal("Failed to retrieve khcheck: ", err)
	}

	// ensure pod spec and containers are set
	if len(c.Spec.RunInterval) == 0 {
		t.Fatal("Found blank RunInterval after setting check UUID")
	}

	if len(c.Spec.PodSpec.Containers) == 0 {
		t.Fatal("Tried to create a test check but the containers list was empty")
	}

	// re-fetch the UUID from the custom resource
	uuid, err := GetWhitelistedUUIDForExternalCheck(c.Namespace, c.Name)
	if err != nil {
		t.Fatal("Failed to get UUID on test check:", err)
	}
	t.Log("Fetched UUID :", uuid)

	if uuid != testUUID {
		t.Fatal("Tried to set and fetch a UUID on a check but the UUID did not match what was set.")
	}

}

// TestGetWhitelistedUUIDForExternalCheck validates that setting
// and fetching whitelist UUIDs works properly
func TestGetWhitelistedUUIDForExternalCheck(t *testing.T) {
	var testUUID = "test-UUID-1234"

	// make an external check and cause it to write a whitelist
	c, err := newTestChecker(client)
	if err != nil {
		t.Fatal("Failed to create new external check:", err)
	}
	c.KubeClient, err = kubeClient.Create(kubeConfigFile)
	if err != nil {
		t.Fatal("Failed to create kube client:", err)
	}

	// delete the UUID (blank it out)
	err = c.setUUID("")
	if err != nil {
		t.Fatal("Failed to blank the UUID on test check:", err)
	}

	// get the UUID's value (should be blank)
	uuid, err := GetWhitelistedUUIDForExternalCheck(c.Namespace, c.Name())
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
	uuid, err = GetWhitelistedUUIDForExternalCheck(c.Namespace, c.Name())
	if err != nil {
		t.Fatal("Failed to get UUID on test check:", err)
	}
	t.Log("Fetched UUID :", uuid)

	if uuid != testUUID {
		t.Fatal("Tried to set and fetch a UUID on a check but the UUID did not match what was set.")
	}
}

func TestSanityCheck(t *testing.T) {
	c, err := newTestChecker(client)
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
	c.CheckName = ""
	err = c.sanityCheck()
	if err == nil {
		t.Fatal(err)
	}

	// fix the pod name and try KubeClient
	c.CheckName = "kuberhealthy"
	c.KubeClient = nil
	err = c.sanityCheck()
	if err == nil {
		t.Fatal(err)
	}

}
