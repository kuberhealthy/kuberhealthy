/*
Copyright 2025 Kuberhealthy Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"log"

	kuberhealthy "github.com/kuberhealthy/kuberhealthy/v3/internal/kuberhealthy"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	khcrdsv2 "github.com/kuberhealthy/crds/api/v2"
)

// KuberhealthyCheckReconciler reconciles KuberhealthyCheck resources
type KuberhealthyCheckReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Kuberhealthy *kuberhealthy.Kuberhealthy
}

func (r *KuberhealthyCheckReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var check khcrdsv2.KuberhealthyCheck
	if err := r.Get(ctx, req.NamespacedName, &check); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	finalizer := "kuberhealthy.com/finalizer"

	// DELETE support for finalizer
	if !check.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&check, finalizer) {
			log.Println("controller: DELETE FINALIZER event detected for:", req.Namespace+"/"+req.Name)
			// Stop Kuberhealthy process before deletion
			r.Kuberhealthy.StopCheck(req.NamespacedName.Namespace, req.NamespacedName.Name) // Stop old instance of check

			// Remove finalizer and update the resource
			controllerutil.RemoveFinalizer(&check, finalizer)
			if err := r.Update(ctx, &check); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer is set
	if !controllerutil.ContainsFinalizer(&check, finalizer) {
		controllerutil.AddFinalizer(&check, finalizer)
		if err := r.Update(ctx, &check); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Optionally update status of the kuberhealthycheck resource
	// check.Status.Phase = "Running"
	if err := r.Status().Update(ctx, &check); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// setupWithManager registers the controller with filtering for create events. This automatically
// starts the manager that is passed in.
func (r *KuberhealthyCheckReconciler) setupWithManager(mgr ctrl.Manager) error {
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
				return false                                                                 // false indicates we do not need to write something to the custom resource
			},
			// DELETE
			// TODO - do we need this DELETE and the one in Reconcile?
			DeleteFunc: func(e event.DeleteEvent) bool {
				log.Println("controller: DELETE event detected for:", e.Object.GetName())
				r.Kuberhealthy.StopCheck(e.Object.GetNamespace(), e.Object.GetName()) // Start new instance of check
				return false                                                          // false indicates we do not need to write something to the custom resource
			},
		}).
		Complete(r)
}

// New creates a new KuberhealthyCheckReconciler with a working controller manager from the kubebuilder packages.
// Expects a kuberhealthy.Kuberhealthy that is already started and runnign to be passed in. If it is not started,
// then this function will start it. First, a Kuberhealthy should be created and started, then this New() func
// should be called with it.
func New(ctx context.Context, kh *kuberhealthy.Kuberhealthy) (*KuberhealthyCheckReconciler, error) {

	// check if kuberhealthy is started
	if !kh.IsStarted() {
		err := kh.Start(ctx)
		if err != nil {
			return nil, fmt.Errorf("controller: error starting kuberhealthy:", err)
		}
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(khcrdsv2.AddToScheme(scheme))

	// Get Kubernetes config
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("controller: error getting kubernetes config: %w", err)
	}

	// Create a new manager
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("controller: error creating manager: %w", err)
	}

	// Create and register the reconciler
	reconciler := &KuberhealthyCheckReconciler{
		Client:       mgr.GetClient(),
		Scheme:       scheme,
		Kuberhealthy: kh,
	}

	if err := reconciler.setupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("controller: error setting up controller with manager: %w", err)
	}

	return reconciler, nil
}
