package podRestarts

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Comcast/kuberhealthy/pkg/kubeClient"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func pod(namespace, image string) *v1.Pod {
	return &v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: namespace}, Spec: v1.PodSpec{Containers: []v1.Container{{Image: image}}}}
}

func TestDoChecks(t *testing.T) {

	client, err := kubeClient.Create(filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	c := New("cloud-squad")
	c.client = client

	ticker := 1
	d := time.Second * 1
	for ticker > 0 {
		err = c.doChecks()
		if err != nil {
			t.Fatal(err)
		}
		up, errors := c.CurrentStatus()
		t.Log("up:", up)
		t.Log("errors:", errors)
		t.Log(time.Now().Format("15:04:05"))
		ticker--
		time.Sleep(d)
	}
}

func TestReapPodRestartChecks(t *testing.T) {

	//Create a new check object for input
	i := New("namespace")

	//Create a new check object for output validation
	o := New("namespace")

	//Generate now so we can have the same time base for every object
	rightNow := time.Now()

	//Input
	//Simulate an old observation on a normal pod with a reset count - unlikely to ever actually occur
	i.RestartObservations["normalPod"] = append(i.RestartObservations["normalPod"], RestartCountObservation{rightNow.Add(time.Duration(-80) * time.Minute), 0})
	//Simulate an old observation on a normal pod
	i.RestartObservations["normalPod"] = append(i.RestartObservations["normalPod"], RestartCountObservation{rightNow.Add(time.Duration(-70) * time.Minute), 1})
	//Simulate a normal observation on a normal pod
	i.RestartObservations["normalPod"] = append(i.RestartObservations["normalPod"], RestartCountObservation{rightNow.Add(time.Duration(-3) * time.Minute), 2})
	i.RestartObservations["normalPod"] = append(i.RestartObservations["normalPod"], RestartCountObservation{rightNow.Add(time.Duration(-2) * time.Minute), 3})
	i.RestartObservations["normalPod"] = append(i.RestartObservations["normalPod"], RestartCountObservation{rightNow.Add(time.Duration(-1) * time.Minute), 4})
	i.RestartObservations["normalPod"] = append(i.RestartObservations["normalPod"], RestartCountObservation{rightNow, 5})

	//Simulate a statefulset pod that has been deleted and recreated with the same name
	i.RestartObservations["ssPod"] = append(i.RestartObservations["ssPod"], RestartCountObservation{rightNow.Add(time.Duration(-3) * time.Minute), 25})
	i.RestartObservations["ssPod"] = append(i.RestartObservations["ssPod"], RestartCountObservation{rightNow.Add(time.Duration(-2) * time.Minute), 26})
	i.RestartObservations["ssPod"] = append(i.RestartObservations["ssPod"], RestartCountObservation{rightNow.Add(time.Duration(-1) * time.Minute), 27})
	i.RestartObservations["ssPod"] = append(i.RestartObservations["ssPod"], RestartCountObservation{rightNow, 0})

	//Simulate a deleted pod
	i.RestartObservations["deletedPod"] = append(i.RestartObservations["deletedPod"], RestartCountObservation{rightNow, 0})

	//Simulate an old observation on another normal pod
	i.RestartObservations["anotherPod"] = append(i.RestartObservations["anotherPod"], RestartCountObservation{rightNow.Add(time.Duration(-70) * time.Minute), 15})
	i.RestartObservations["anotherPod"] = append(i.RestartObservations["anotherPod"], RestartCountObservation{rightNow.Add(time.Duration(-62) * time.Minute), 16})
	i.RestartObservations["anotherPod"] = append(i.RestartObservations["anotherPod"], RestartCountObservation{rightNow.Add(time.Duration(-61) * time.Minute), 16})
	//Simulate a normal observation on a another normal pod
	i.RestartObservations["anotherPod"] = append(i.RestartObservations["anotherPod"], RestartCountObservation{rightNow.Add(time.Duration(-3) * time.Minute), 17})
	i.RestartObservations["anotherPod"] = append(i.RestartObservations["anotherPod"], RestartCountObservation{rightNow.Add(time.Duration(-2) * time.Minute), 18})
	i.RestartObservations["anotherPod"] = append(i.RestartObservations["anotherPod"], RestartCountObservation{rightNow.Add(time.Duration(-1) * time.Minute), 19})
	i.RestartObservations["anotherPod"] = append(i.RestartObservations["anotherPod"], RestartCountObservation{rightNow, 20})

	//Output
	//normalPod
	o.RestartObservations["normalPod"] = append(o.RestartObservations["normalPod"], RestartCountObservation{rightNow.Add(time.Duration(-3) * time.Minute), 2})
	o.RestartObservations["normalPod"] = append(o.RestartObservations["normalPod"], RestartCountObservation{rightNow.Add(time.Duration(-2) * time.Minute), 3})
	o.RestartObservations["normalPod"] = append(o.RestartObservations["normalPod"], RestartCountObservation{rightNow.Add(time.Duration(-1) * time.Minute), 4})
	o.RestartObservations["normalPod"] = append(o.RestartObservations["normalPod"], RestartCountObservation{rightNow, 5})

	//ssPod
	o.RestartObservations["ssPod"] = append(o.RestartObservations["ssPod"], RestartCountObservation{rightNow, 0})

	//anotherPod
	o.RestartObservations["anotherPod"] = append(o.RestartObservations["anotherPod"], RestartCountObservation{rightNow.Add(time.Duration(-3) * time.Minute), 17})
	o.RestartObservations["anotherPod"] = append(o.RestartObservations["anotherPod"], RestartCountObservation{rightNow.Add(time.Duration(-2) * time.Minute), 18})
	o.RestartObservations["anotherPod"] = append(o.RestartObservations["anotherPod"], RestartCountObservation{rightNow.Add(time.Duration(-1) * time.Minute), 19})
	o.RestartObservations["anotherPod"] = append(o.RestartObservations["anotherPod"], RestartCountObservation{rightNow, 20})

	//Simulate a list of existing pods returned from a pod list
	l := &v1.PodList{
		Items: []v1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "normalPod",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ssPod",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "anotherPod",
				},
			},
		},
	}

	//Dont fear the reaper
	i.reapPodRestartChecks(l)

	// Compare the pod lists of the input and output data and verify they are the same aka expected
	t.Log("After")
	for outPodName, _ := range o.RestartObservations {
		for inPodName, _ := range i.RestartObservations {
			if outPodName == inPodName {
				// Compare the values of the pod observation slices and look for differences so we can specifically call out
				// which pods were improperly reaped
				if !reflect.DeepEqual(o.RestartObservations[inPodName], i.RestartObservations[outPodName]) {
					t.Log("Test failed, reaper failed to generate expected output for pod:", inPodName)
					t.Fail()
				}
			}
		}
	}
	//Compare the entire maps so we catch deleted pod reaping failures and any other possible issues
	if !reflect.DeepEqual(o.RestartObservations, i.RestartObservations) {
		t.Log("Test failed, reaper failed to generate expected output.")
		t.Fail()
	}
}
