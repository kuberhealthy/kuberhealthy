package controller

import (
	"fmt"
	"log"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	khcrdsv2 "github.com/kuberhealthy/crds/api/v2"
)

// setupWithManager registers the controller with filtering for create events. This automatically
// starts the manager that is passed in.
func (r *KuberhealthyCheckReconciler) setupWithManager(mgr ctrl.Manager) error {
	fmt.Println("-- controller setupWithManager")
	return ctrl.NewControllerManagedBy(mgr).
		For(&khcrdsv2.KuberhealthyCheck{}).
		WithEventFilter(predicate.Funcs{
			// CREATE
			CreateFunc: func(e event.CreateEvent) bool {
				log.Println("controller: CREATE event detected for:", e.Object.GetName())
				r.Kuberhealthy.StartCheck(e.Object.GetNamespace(), e.Object.GetName()) // Start new instance of check
				return true                                                            // true indicates we need to write something to the custom resource
			},
			// UPDATE
			UpdateFunc: func(e event.UpdateEvent) bool {
				log.Println("controller: UPDATE event detected for:", e.ObjectOld.GetName())
				r.Kuberhealthy.StopCheck(e.ObjectOld.GetNamespace(), e.ObjectOld.GetName())  // Start new instance of check
				r.Kuberhealthy.StartCheck(e.ObjectNew.GetNamespace(), e.ObjectNew.GetName()) // Start new instance of check
				return true                                                                  // false indicates we do not need to write something to the custom resource
				// return true // TODO - why do delete events come in as UPDATE?
			},
			// DELETE
			// TODO - do we need this DELETE and the one in Reconcile?
			DeleteFunc: func(e event.DeleteEvent) bool {
				log.Println("controller: DELETE event detected for:", e.Object.GetName())
				r.Kuberhealthy.StopCheck(e.Object.GetNamespace(), e.Object.GetName()) // Start new instance of check
				return true                                                           // we return true to indicate that we must write something back to the cusotm resource, such as removing the finalizer
			},
		}).
		Complete(r)
}
