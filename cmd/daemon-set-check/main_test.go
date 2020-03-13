package main

import (
	"reflect"
	"strconv"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"
)

func testSetup(clientNeeded bool) (*Checker, error) {
	CheckRunTime = time.Now().Unix()
	Namespace = "kuberhealthy"

	var err error

	dsc, err := New()

	if err != nil {
		log.Fatalln("Unable to create daemon set checker", err)
	}

	if clientNeeded {
		client, err := kubeClient.Create(KubeConfigFile)
		if err != nil {
			log.Fatalln("Unable to create kubernetes client", err)
		}
		dsc.client = client
	}
	return dsc, err
}

func TestGenerateDaemonSetSpec(t *testing.T) {
	dsc, err := testSetup(false)
	if err != nil {
		log.Fatalln("Failed to create test setup", err)
	}

	dsc.Tolerations = []apiv1.Toleration{{Key: "node-role.kubernetes.io/master", Effect: apiv1.TaintEffect("NoSchedule")}}
	dsc.generateDaemonSetSpec()

	if dsc.DaemonSet == nil || dsc.DaemonSet.Name != dsc.DaemonSetName {
		t.Fatalf("Daemonset was not correctly generated")
	}
	t.Logf("Daemonset: %s has been correctly generated: %v", dsc.DaemonSetName, dsc.DaemonSet)
}

func TestCleanupOrphans(t *testing.T) {
	t.Skip("Skipping cause this test requires the kubernetes client")

	checker, err := testSetup(true)
	if err != nil {
		t.Fatal(err)
	}

	checker.deleteDS(checker.DaemonSetName)

	err = makeDaemonsets(checker, true)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)

	err = checker.cleanupOrphans()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPauseContainerOverride(t *testing.T) {
	// verify that we are getting the expected default value from a new dsc
	dsc, err := testSetup(false)
	if err != nil {
		t.Fatal(err)
	}
	if dsc.PauseContainerImage != "gcr.io/google-containers/pause:3.1" {
		t.Fatal("Default Pause Container Image is not set or an unexpected value, actual value:", dsc.PauseContainerImage)
	}

	dscO, err := testSetup(false)
	if err != nil {
		t.Fatal(err)
	}
	// Silly, yes, but this mimics how the program is setting the override value
	dscO.PauseContainerImage = "gcr.io/google-containers/pause:3.1"
	if dscO.PauseContainerImage != "gcr.io/google-containers/pause:3.1" {
		t.Fatal("Overridden Pause Container Image is not set or an unexpected value, actual value:", dscO.PauseContainerImage)
	}
}

func TestGetAllAndDeleteDaemonsetsAndPods(t *testing.T) {
	t.Skip("Skipping cause this test requires the kubernetes client")

	checker, err := testSetup(true)
	if err != nil {
		t.Fatal(err)
	}

	// Creates a single daemonset that also generates daemonset pods
	err = makeDaemonsets(checker, false)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for daemonset and pods to come up
	time.Sleep(time.Second * 20)

	dsList, err := checker.getAllDaemonsets()
	if err != nil {
		t.Fatal(err)
	}

	podList, err := checker.getAllPods()
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range podList {
		t.Logf("Found pod: %s", p.Name)

		// Clean up
		err := checker.deletePod(p.Name)
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, ds := range dsList {
		t.Logf("Found daemonset: %s", ds.Name)
		time.Sleep(time.Second)

		// Clean up
		err := checker.deleteDS(ds.Name)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Wait for daemonset and pods to delete
	time.Sleep(time.Second * 100)

	postPodList, err := checker.getAllPods()
	if err != nil {
		t.Fatal(err)
	}

	if len(postPodList) > 0 {
		for _, p := range postPodList {
			t.Logf("Found post pod: %s", p.Name)
		}
		t.Fatal("Daemonset pods have not been deleted / cleaned up.")
	}
	t.Logf("Daemonset pods have been deleted / cleaned up.")

	postDSList, err := checker.getAllDaemonsets()
	if err != nil {
		t.Fatal(err)
	}

	if len(postDSList) > 0 {
		for _, ds := range postDSList {
			t.Logf("Found post daemonset: %s", ds.Name)
		}
		t.Fatal("Daemonset has not been deleted / cleaned up.")
	}
	t.Logf("Daemonset has been deleted / cleaned up.")
}

func TestParseTolerationOverride(t *testing.T) {
	var taintTests = []struct {
		input    []string           // input
		expected []apiv1.Toleration //output
		err      string
	}{
		{[]string{"node-role.kubernetes.io/master,,NoSchedule"},
			[]apiv1.Toleration{
				{
					Key:    "node-role.kubernetes.io/master",
					Value:  "",
					Effect: apiv1.TaintEffect("NoSchedule"),
				},
			},
			"",
		},
		{
			[]string{"dedicated,someteam,NoSchedule"},
			[]apiv1.Toleration{
				{
					Key:    "dedicated",
					Value:  "someteam",
					Effect: apiv1.TaintEffect("NoSchedule"),
				},
			},
			"",
		},
		{[]string{"node-role.kubernetes.io/master,,NoSchedule", "dedicated,someteam,NoSchedule"},
			[]apiv1.Toleration{
				{
					Key:    "node-role.kubernetes.io/master",
					Value:  "",
					Effect: apiv1.TaintEffect("NoSchedule"),
				},
				{
					Key:    "dedicated",
					Value:  "someteam",
					Effect: apiv1.TaintEffect("NoSchedule"),
				},
			},
			"",
		},
		{[]string{"too,much,input,for,this,function"}, []apiv1.Toleration{},
			"Unable to parse the passed in taint overrides - are they in the correct format?",
		},
		{[]string{"notenoughinput"}, []apiv1.Toleration{},
			"Unable to parse the passed in taint overrides - are they in the correct format?",
		},
	}

	checker, err := testSetup(false)
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range taintTests {
		actual, err := checker.ParseTolerationOverride(tt.input)
		if err != nil && err.Error() != tt.err {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, tt.expected) {
			t.Error("Failure! - Input:", tt.input, "Expected:", tt.expected, "Received:", actual, "Error:", err)
			continue
		}
		t.Log("Success! - Input:", tt.input, "Expected:", tt.expected, "Received:", actual, "Error:", err)
	}
}

// makeDaemonsets creates a daemonset that can also be an orphaned daemonset
func makeDaemonsets(dsc *Checker, orphan bool) error {

	if orphan {
		dsc.hostname = "ORPHANED-TEST"
	}

	checkRunTime := strconv.Itoa(int(CheckRunTime))
	hostname := getHostname()

	terminationGracePeriod := int64(1)
	testDS := Checker{
		Namespace:     Namespace,
		DaemonSetName: daemonSetBaseName + "-" + hostname + "-" + checkRunTime,
		hostname:      hostname,
	}

	testDS.DaemonSet = &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: testDS.DaemonSetName,
			Labels: map[string]string{
				"app":              testDS.DaemonSetName,
				"source":           "kuberhealthy",
				"creatingInstance": dsc.hostname,
				"checkRunTime":     checkRunTime,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":              testDS.DaemonSetName,
					"source":           "kuberhealthy",
					"creatingInstance": dsc.hostname,
					"checkRunTime":     checkRunTime,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":              testDS.DaemonSetName,
						"source":           "kuberhealthy",
						"creatingInstance": dsc.hostname,
						"checkRunTime":     checkRunTime,
					},
					Name: testDS.DaemonSetName,
				},
				Spec: apiv1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					Tolerations: []apiv1.Toleration{
						{
							Key:    "node-role.kubernetes.io/master",
							Effect: "NoSchedule",
						},
					},
					Containers: []apiv1.Container{
						{
							Name:  "sleep",
							Image: "gcr.io/google_containers/pause:0.8.0",
							Resources: apiv1.ResourceRequirements{
								Requests: apiv1.ResourceList{
									apiv1.ResourceCPU:    resource.MustParse("0"),
									apiv1.ResourceMemory: resource.MustParse("0"),
								},
							},
						},
					},
				},
			},
		},
	}

	daemonSetClient := dsc.getDaemonSetClient()
	_, err := daemonSetClient.Create(testDS.DaemonSet)
	return err
}
