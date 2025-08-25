package controller

import (
	"fmt"
	"reflect"

	log "github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	khcrdsv2 "github.com/kuberhealthy/crds/api/v2"
)

// setupWithManager registers the controller with filtering for create events. This automatically
// starts the manager that is passed in.
func (r *KuberhealthyCheckReconciler) setupWithManager(mgr ctrl.Manager) error {
	log.Debugln("controller: setupWithManager")
	return ctrl.NewControllerManagedBy(mgr).
		For(&khcrdsv2.KuberhealthyCheck{}).
		WithEventFilter(predicate.Funcs{
			// CREATE
			CreateFunc: func(e event.CreateEvent) bool {
				log.Infoln("controller: CREATE event detected for:", e.Object.GetName())
				khcheck, err := convertToKHCheck(e.Object)
				if err != nil {
					log.Errorln("error:", err.Error())
					return false
				}
				err = r.Kuberhealthy.StartCheck(khcheck) // Start new instance of check
				return true                              // true indicates we need to write something to the custom resource
			},
			// UPDATE
			UpdateFunc: func(e event.UpdateEvent) bool {
				log.Infoln("controller: UPDATE event detected for:", e.ObjectOld.GetName())
				oldKHCheck, err := convertToKHCheck(e.ObjectOld)
				if err != nil {
					log.Errorln("error:", err.Error())
					return false
				}
				newKHCheck, err := convertToKHCheck(e.ObjectNew)
				if err != nil {
					log.Errorln("error:", err.Error())
					return false
				}
				r.Kuberhealthy.UpdateCheck(oldKHCheck, newKHCheck)
				return true // true here means that we need to write changes back to the CRD
			},
			// DELETE
			// TODO - do we need this DELETE and the one in Reconcile?
			DeleteFunc: func(e event.DeleteEvent) bool {
				log.Infoln("controller: DELETE event detected for:", e.Object.GetName())
				khcheck, err := convertToKHCheck(e.Object)
				if err != nil {
					log.Errorln("error:", err.Error())
					return false
				}
				err = r.Kuberhealthy.StopCheck(khcheck) // Start new instance of check
				return true                             // we return true to indicate that we must write something back to the cusotm resource, such as removing the finalizer
			},
		}).
		Complete(r)
}

// convertToKHChecks casts the old and new objects to KuberhealthyCheck CRDs
func convertToKHCheck(obj client.Object) (*khcrdsv2.KuberhealthyCheck, error) {
	khcheck, ok := obj.(*khcrdsv2.KuberhealthyCheck)
	if !ok {
		actualType := reflect.TypeOf(obj)
		return nil, fmt.Errorf("unexpected object type recieved by controller for khcheck: %s", actualType.String())
	}

	return khcheck, nil
}
