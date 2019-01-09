package podRestarts

import (
	"github.com/Comcast/kuberhealthy/pkg/kubeClient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/api/core/v1"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func pod(namespace, image string) *v1.Pod {
	return &v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: namespace}, Spec: v1.PodSpec{Containers: []v1.Container{{Image: image}}}}
}

func TestDoChecks(t *testing.T) {

	client, err := kubeClient.Create(filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	c := New("cloud-squad")
	c.client = client

	ticker := 1
	d := time.Second*1
	for ticker >0 {
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

func TestReapPodRestartChecks(t *testing.T){

	//Create a new check object
	c := New("namespace")

	//Simulate an old observation on a normal pod with a reset count - unlikely to ever actually occur
	c.RestartObservations["normalPod"] = append(c.RestartObservations["normalPod"], RestartCountObservation{time.Now().Add(time.Duration(-80)*time.Minute), 0})
	//Simulate an old observation on a normal pod
	c.RestartObservations["normalPod"] = append(c.RestartObservations["normalPod"], RestartCountObservation{time.Now().Add(time.Duration(-70)*time.Minute), 1})
	//Simulate a normal observation on a normal pod
	c.RestartObservations["normalPod"] = append(c.RestartObservations["normalPod"], RestartCountObservation{time.Now().Add(time.Duration(-3)*time.Minute), 2})
	c.RestartObservations["normalPod"] = append(c.RestartObservations["normalPod"], RestartCountObservation{time.Now().Add(time.Duration(-2)*time.Minute), 3})
	c.RestartObservations["normalPod"] = append(c.RestartObservations["normalPod"], RestartCountObservation{time.Now().Add(time.Duration(-1)*time.Minute), 4})
	c.RestartObservations["normalPod"] = append(c.RestartObservations["normalPod"], RestartCountObservation{time.Now(), 5})

	//Simulate a statefulset pod that has been deleted and recreated with the same name
	c.RestartObservations["ssPod"] = append(c.RestartObservations["ssPod"], RestartCountObservation{time.Now().Add(time.Duration(-3)*time.Minute), 25})
	c.RestartObservations["ssPod"] = append(c.RestartObservations["ssPod"], RestartCountObservation{time.Now().Add(time.Duration(-2)*time.Minute), 26})
	c.RestartObservations["ssPod"] = append(c.RestartObservations["ssPod"], RestartCountObservation{time.Now().Add(time.Duration(-1)*time.Minute), 27})
	c.RestartObservations["ssPod"] = append(c.RestartObservations["ssPod"], RestartCountObservation{time.Now(), 0})

	//Simulate a deleted pod
	c.RestartObservations["deletedPod"] = append(c.RestartObservations["deletedPod"], RestartCountObservation{time.Now(), 0})

	//Simulate an old observation on another normal pod
	c.RestartObservations["anotherPod"] = append(c.RestartObservations["anotherPod"], RestartCountObservation{time.Now().Add(time.Duration(-70)*time.Minute), 15})
	c.RestartObservations["anotherPod"] = append(c.RestartObservations["anotherPod"], RestartCountObservation{time.Now().Add(time.Duration(-62)*time.Minute), 16})
	c.RestartObservations["anotherPod"] = append(c.RestartObservations["anotherPod"], RestartCountObservation{time.Now().Add(time.Duration(-61)*time.Minute), 16})
	//Simulate a normal observation on a another normal pod
	c.RestartObservations["anotherPod"] = append(c.RestartObservations["anotherPod"], RestartCountObservation{time.Now().Add(time.Duration(-3)*time.Minute), 17})
	c.RestartObservations["anotherPod"] = append(c.RestartObservations["anotherPod"], RestartCountObservation{time.Now().Add(time.Duration(-2)*time.Minute), 18})
	c.RestartObservations["anotherPod"] = append(c.RestartObservations["anotherPod"], RestartCountObservation{time.Now().Add(time.Duration(-1)*time.Minute), 19})
	c.RestartObservations["anotherPod"] = append(c.RestartObservations["anotherPod"], RestartCountObservation{time.Now(), 20})


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

	//Pods that existed before the reap
	t.Log("Before")
	for podName, restartObservations := range c.RestartObservations {
		t.Log(podName)
		for _, observation := range restartObservations{
			t.Log(observation)
		}
	}

	//Dont fear the reaper
	c.reapPodRestartChecks(l)

	//Pods that existed after the reap
	//TODO Replace with an actual programatic check vs this eyeball test
	//TODO check that each pod group has the expected values after reaping and that the mostRecent and second most Recent
	//TODO values are correctly filled out
	t.Log("After")
	for podName, restartObservations := range c.RestartObservations {
		t.Log(podName)
		for _, observation := range restartObservations{
			t.Log(observation)
		}
	}
}