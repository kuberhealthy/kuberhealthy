package external

import (
	"errors"
	"testing"

	khcrds "github.com/kuberhealthy/kuberhealthy/v3/pkg/apis/comcast.github.io/v1"
	fake "k8s.io/client-go/kubernetes/fake"
)

var fakeClient *fake.Clientset

const defaultNamespace = "kuberhealthy"

func init() {

	// init a fake kubernetes client for testing
	// kubernetesClient = kubernetes.NewForConfigOrDie(restConfig)
	fakeClient = fake.NewSimpleClientset()

}

// newTestChecker creates a new test checker struct with a basic set of defaults
// that work out of the box
func newTestChecker() (*Checker, error) {
	podCheckFile := "test/basicCheckerPod.yaml"
	p, err := loadTestPodSpecFile(podCheckFile)
	if err != nil {
		return &Checker{}, errors.New("Unable to load kubernetes pod spec " + podCheckFile + " " + err.Error())
	}
	chk := newTestCheckFromSpec(p, DefaultKuberhealthyReportingURL)
	return chk, err
}

// newTestCheckFromSpec creates a new test checker but using the supplied
// spec file for a khcheck
func newTestCheckFromSpec(checkSpec *khcrds.KuberhealthyCheck, reportingURL string) *Checker {
	// create a new checker and insert this pod spec
	checker := New(fakeClient, checkSpec, nil, reportingURL) // external checker does not ever return an error so we drop it
	checker.Debug = true
	return checker
}

// TestExternalCheckerSanitation tests the external checker in a situation
// where it should fail
// func TestExternalCheckerSanitation(t *testing.T) {
// 	t.Parallel()

// 	// make a new default checker of this check
// 	checker, err := newTestChecker()
// 	if err != nil {
// 		t.Fatal("Failed to create client:", err)
// 	}
// 	checker.KubeClientInterface = fakeClient

// 	// sabotage the pod name
// 	checker.CheckName = ""

// 	// run the checker with the kube client
// 	err = checker.RunOnce(context.Background())
// 	if err == nil {
// 		t.Fatal("Expected pod name blank validation check failure but did not hit it")
// 	}
// 	t.Log("got expected error:", err)

// 	// break the pod namespace instead now
// 	checker.CheckName = DefaultName
// 	checker.Namespace = ""

// 	// run the checker with the kube client
// 	err = checker.RunOnce(context.Background())
// 	if err == nil {
// 		t.Fatal("Expected namespace blank validation check failure but did not hit it")
// 	}
// 	t.Log("got expected error:", err)

// 	// break the pod spec now instead of namespace
// 	checker.Namespace = defaultNamespace
// 	checker.PodSpec = apiv1.PodSpec{}

// 	// run the checker with the kube client
// 	err = checker.RunOnce(context.Background())
// 	if err == nil {
// 		t.Fatal("Expected pod validation check failure but did not hit it")
// 	}
// 	t.Log("got expected error:", err)
// }

func TestSanityCheck(t *testing.T) {
	c, err := newTestChecker()
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

	// fix the namespace, then try blanking the check name
	c.Namespace = defaultNamespace
	c.CheckName = ""
	err = c.sanityCheck()
	if err == nil {
		t.Fatal(err)
	}

	// fix the pod name and try KubeClient
	c.CheckName = "kuberhealthy"
	c.KubeClientInterface = nil
	err = c.sanityCheck()
	if err == nil {
		t.Fatal(err)
	}

}
