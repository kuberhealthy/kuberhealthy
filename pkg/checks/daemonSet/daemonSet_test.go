package daemonSet

import (
	"strconv"
	"testing"
	"time"

	"github.com/Comcast/kuberhealthy/pkg/kubeClient"
	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	betaapiv1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const kubeConfigFile = "~/.kube/config"

func TestCleanupOrphans(t *testing.T) {
	checker, err := New()
	if err != nil {
		t.Fatal(err)
	}

	checker.deleteDS(checker.DaemonSetName)

	err = makeOrphan(checker)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)

	err = checker.cleanupOrphans()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetAllDaemonsets(t *testing.T) {
	checker, err := New()
	if err != nil {
		t.Fatal(err)
	}
	dsList, err := checker.getAllDaemonsets()
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

	err = checker.Run(client)
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

	testDS.DaemonSet = &betaapiv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: testDS.DaemonSetName,
			Labels: map[string]string{
				"app":              testDS.DaemonSetName,
				"source":           "kuberhealthy",
				"creatingInstance": "ORPHANED-TEST",
			},
		},
		Spec: betaapiv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":              testDS.DaemonSetName,
					"source":           "kuberhealthy",
					"creatingInstance": "ORPHANED-TEST",
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":              testDS.DaemonSetName,
						"source":           "kuberhealthy",
						"creatingInstance": "ORPHANED-TEST",
					},
					Name: testDS.DaemonSetName,
				},
				Spec: apiv1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					Tolerations: []apiv1.Toleration{
						apiv1.Toleration{
							Key:    "node-role.kubernetes.io/master",
							Effect: "NoSchedule",
						},
					},
					Containers: []apiv1.Container{
						apiv1.Container{
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
	_, err := daemonSetClient.Create(dsc.DaemonSet)
	return err
}
