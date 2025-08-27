package controller

import (
	"fmt"

	khcrdsv2 "github.com/kuberhealthy/crds/api/v2"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

// setupWithManager attaches event handlers to the shared informer so that
// Kuberhealthy checks start, update, or stop when their resources change.
func (c *KHCheckController) setupWithManager() {
	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handleCreate,
		UpdateFunc: c.handleUpdate,
		DeleteFunc: c.handleDelete,
	})
}

func (c *KHCheckController) handleCreate(obj interface{}) {
	kh, err := convertToKHCheck(obj)
	if err != nil {
		log.Errorln("error:", err.Error())
		return
	}
	if err := c.Kuberhealthy.StartCheck(kh); err != nil {
		log.Errorln("error:", err.Error())
	}
}

func (c *KHCheckController) handleUpdate(oldObj, newObj interface{}) {
	oldCheck, err := convertToKHCheck(oldObj)
	if err != nil {
		log.Errorln("error:", err.Error())
		return
	}
	newCheck, err := convertToKHCheck(newObj)
	if err != nil {
		log.Errorln("error:", err.Error())
		return
	}
	c.Kuberhealthy.UpdateCheck(oldCheck, newCheck)
}

func (c *KHCheckController) handleDelete(obj interface{}) {
	kh, err := convertToKHCheck(obj)
	if err != nil {
		log.Errorln("error:", err.Error())
		return
	}
	if err := c.Kuberhealthy.StopCheck(kh); err != nil {
		log.Errorln("error:", err.Error())
	}
}

// convertToKHCheck converts an informer object into a KuberhealthyCheck.
func convertToKHCheck(obj interface{}) (*khcrdsv2.KuberhealthyCheck, error) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("unexpected object type %T", obj)
	}
	kh := &khcrdsv2.KuberhealthyCheck{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, kh); err != nil {
		return nil, err
	}
	return kh, nil
}
