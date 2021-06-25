package main

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	khstatev1 "github.com/kuberhealthy/kuberhealthy/v2/pkg/apis/khstate/v1"
)

type testLW struct {
	ListFunc  func(options metav1.ListOptions) (runtime.Object, error)
	WatchFunc func(options metav1.ListOptions) (watch.Interface, error)
}

func (t *testLW) List(options metav1.ListOptions) (runtime.Object, error) {
	return t.ListFunc(options)
}
func (t *testLW) Watch(options metav1.ListOptions) (watch.Interface, error) {
	return t.WatchFunc(options)
}

// makeTestStateReflector creates a test StateReflector object
func makeTestStateReflector(fakeWatcher *watch.FakeWatcher) *StateReflector {
	sr := StateReflector{}
	sr.reflectorSigChan = make(chan struct{})
	sr.resyncPeriod = time.Minute * 5

	// structure the reflector and its required elements
	sr.store = cache.NewStore(cache.MetaNamespaceKeyFunc)
	listerWatcher := &testLW{
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return fakeWatcher, nil
		},
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return &khstatev1.KuberhealthyStateList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}}, nil
		},
	}

	sr.reflector = cache.NewReflector(listerWatcher, &khstatev1.KuberhealthyState{}, sr.store, sr.resyncPeriod)

	return &sr
}

// TestStart ensures that the KuberhealthyStateReflector starts properly
func TestStart(t *testing.T) {

	fw := watch.NewFake()
	khStateReflector := makeTestStateReflector(fw)
	go khStateReflector.Start()

	fw.Add(&khstatev1.KuberhealthyState{ObjectMeta: metav1.ObjectMeta{Name: "bar"}})
	close(khStateReflector.reflectorSigChan)
	select {
	case _, ok := <-fw.ResultChan():
		if ok {
			t.Errorf("Watch channel left open after stopping the watch")
		}
	case <-time.After(wait.ForeverTestTimeout):
		t.Errorf("the cancellation is at least %s late", wait.ForeverTestTimeout.String())
		break
	}

}

// TestStop ensures that the KuberhealthyStateReflector stops properly after it starts
func TestStop(t *testing.T) {
	fw := watch.NewFake()
	khStateReflector := makeTestStateReflector(fw)

	// Run watch for 5 seconds
	go khStateReflector.Start()
	fw.Add(&khstatev1.KuberhealthyState{ObjectMeta: metav1.ObjectMeta{Name: "bar"}})
	time.Sleep(time.Second * 2)

	// Stop watch.
	khStateReflector.Stop()

	select {
	case _, ok := <-fw.ResultChan():
		if ok {
			t.Errorf("Watch channel left open after stopping the watch")
		}
	case <-time.After(wait.ForeverTestTimeout):
		t.Errorf("the cancellation is at least %s late", wait.ForeverTestTimeout.String())
		break
	}
}
