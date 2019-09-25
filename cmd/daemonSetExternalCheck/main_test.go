package main

import (
	"reflect"
	"strconv"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	betaapiv1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Comcast/kuberhealthy/pkg/kubeClient"
)

const kubeConfigFile = "~/.kube/config"

func TestCleanupOrphans(t *testing.T) {
	checker, err := New()
	if err != nil {
		t.Fatal(err)
	}

	deleteDS(DaemonSetName)

	err = makeOrphan(checker)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)

	err = cleanupOrphans()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPauseContainerOverride(t *testing.T) {
	// verify that we are getting the expected default value from a new dsc
	dsc, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if PauseContainerImage != "gcr.io/google_containers/pause:0.8.0" {
		t.Fatal("Default Pause Container Image is not set or an unexpected value, actual value:", PauseContainerImage)
	}

	dscO, err := New()
	if err != nil {
		t.Fatal(err)
	}
	// Silly, yes, but this mimics how the program is setting the override value
	PauseContainerImage = "another-image-repo/pause:0.8.0"
	if PauseContainerImage != "another-image-repo/pause:0.8.0" {
		t.Fatal("Overridden Pause Container Image is not set or an unexpected value, actual value:", PauseContainerImage)
	}
}

func TestGetAllDaemonsets(t *testing.T) {
	checker, err := New()
	if err != nil {
		t.Fatal(err)
	}
	dsList, err := getAllDaemonsets()
	if err != nil {
		t.Fatal(err)
	}
	for _, ds := range dsList {
		t.Log(ds.Name)
	}
}

func TestChecker(t *testing.T) {

	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		log.Errorln("Unable to create kubernetes client", err)
	}

	checker, err := New()
	if err != nil {
		t.Fatal(err)
	}

	err = Run(client)
	if err != nil {
		t.Fatal(err)
	}

}

// makeOrphan creates an orphaned daemonset
func makeOrphan(dsc *Checker) error {

	hostname := getHostname()

	terminationGracePeriod := int64(1)
	testDS := Checker{
		Namespace:     namespace,
		DaemonSetName: daemonSetBaseName + "-" + hostname + "-" + strconv.Itoa(int(time.Now().Unix())),
		hostname:      hostname,
	}

	DaemonSet = &betaapiv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: DaemonSetName,
			Labels: map[string]string{
				"app":              DaemonSetName,
				"source":           "kuberhealthy",
				"creatingInstance": "ORPHANED-TEST",
			},
		},
		Spec: betaapiv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":              DaemonSetName,
					"source":           "kuberhealthy",
					"creatingInstance": "ORPHANED-TEST",
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":              DaemonSetName,
						"source":           "kuberhealthy",
						"creatingInstance": "ORPHANED-TEST",
					},
					Name: DaemonSetName,
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

	daemonSetClient := getDaemonSetClient()
	_, err := daemonSetClient.Create(DaemonSet)
	return err
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

	checker, err := New()
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range taintTests {
		actual, err := ParseTolerationOverride(tt.input)
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
