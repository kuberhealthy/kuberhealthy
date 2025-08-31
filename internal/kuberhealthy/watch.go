package kuberhealthy

import (
	"fmt"

	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynamicinformer "k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// startKHCheckWatch begins watching KuberhealthyCheck resources and reacts to events.
func (kh *Kuberhealthy) startKHCheckWatch(cfg *rest.Config) {
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		log.Errorln("kuberhealthy: failed to create dynamic client:", err)
		return
	}
	gvr := schema.GroupVersionResource{Group: "kuberhealthy.github.io", Version: "v2", Resource: "kuberhealthychecks"}
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dyn, 0, metav1.NamespaceAll, nil)
	inf := factory.ForResource(gvr).Informer()
	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    kh.handleCreate,
		UpdateFunc: func(oldObj, newObj interface{}) { kh.handleUpdate(oldObj, newObj) },
		DeleteFunc: kh.handleDelete,
	})
	go inf.Run(kh.Context.Done())
}

func (kh *Kuberhealthy) handleCreate(obj interface{}) {
	khc, err := convertToKHCheck(obj)
	if err != nil {
		log.Errorln("error:", err)
		return
	}
	if err := kh.addFinalizer(kh.Context, khc); err != nil {
		log.Errorln("error:", err)
		return
	}
	if err := kh.StartCheck(khc); err != nil {
		log.Errorln("error:", err)
	}
}

func (kh *Kuberhealthy) handleUpdate(oldObj, newObj interface{}) {
	oldCheck, err := convertToKHCheck(oldObj)
	if err != nil {
		log.Errorln("error:", err)
		return
	}
	newCheck, err := convertToKHCheck(newObj)
	if err != nil {
		log.Errorln("error:", err)
		return
	}
	if !newCheck.GetDeletionTimestamp().IsZero() {
		if kh.hasFinalizer(newCheck) {
			if err := kh.StopCheck(newCheck); err != nil {
				log.Errorln("error:", err)
			}
			refreshed := &khapi.KuberhealthyCheck{}
			nn := types.NamespacedName{Namespace: newCheck.Namespace, Name: newCheck.Name}
			if err := kh.CheckClient.Get(kh.Context, nn, refreshed); err != nil {
				log.Errorln("error:", err)
				return
			}
			if err := kh.deleteFinalizer(kh.Context, refreshed); err != nil {
				log.Errorln("error:", err)
			}
		}
		return
	}
	kh.UpdateCheck(oldCheck, newCheck)
}

func (kh *Kuberhealthy) handleDelete(obj interface{}) {
	khc, err := convertToKHCheck(obj)
	if err != nil {
		log.Errorln("error:", err)
		return
	}
	if err := kh.StopCheck(khc); err != nil {
		log.Errorln("error:", err)
	}
}

func convertToKHCheck(obj interface{}) (*khapi.KuberhealthyCheck, error) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("unexpected object type %T", obj)
	}
	khc := &khapi.KuberhealthyCheck{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, khc); err != nil {
		return nil, err
	}
	return khc, nil
}
