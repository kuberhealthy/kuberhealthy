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

	kuberhealthygithubiov4 "github.com/kuberhealthy/crds/api/v4"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// KuberhealthyJobReconciler reconciles a KuberhealthyJob object
type KuberhealthyJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kuberhealthy.github.io.kuberhealthy.github.io,resources=kuberhealthyjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kuberhealthy.github.io.kuberhealthy.github.io,resources=kuberhealthyjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kuberhealthy.github.io.kuberhealthy.github.io,resources=kuberhealthyjobs/finalizers,verbs=update

// SetupWithManager sets up the controller with the Manager.
func (r *KuberhealthyJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kuberhealthygithubiov4.KuberhealthyJob{}).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KuberhealthyJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
func (r *KuberhealthyJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	log := log.FromContext(ctx)
	log.Info("Reconciling KuberhealthyJob", "name", req.NamespacedName)

	// Fetch the KuberhealthyCheck instance
	var khCheck kuberhealthygithubiov4.KuberhealthyCheck
	err := r.Get(ctx, req.NamespacedName, &khCheck)

	if err != nil {
		if errors.IsNotFound(err) {
			// Handle Delete event
			return r.handleDelete(ctx, req)
		}
		log.Error(err, "Failed to get KuberhealthyJob")
		return ctrl.Result{}, err
	}

	// Determine if it's a Create or Update
	if khCheck.CreationTimestamp.IsZero() {
		return r.handleCreate(ctx, &khCheck)
	} else {
		return r.handleUpdate(ctx, &khCheck)
	}
}

// Handle Create Event
func (r *KuberhealthyJobReconciler) handleCreate(ctx context.Context, khCheck *kuberhealthygithubiov4.KuberhealthyCheck) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Handling Create Event", "name", khCheck.Name)

	// TODO: Add logic for creation
	return ctrl.Result{}, nil
}

// Handle Update Event
func (r *KuberhealthyJobReconciler) handleUpdate(ctx context.Context, khCheck *kuberhealthygithubiov4.KuberhealthyCheck) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Handling Update Event", "name", khCheck.Name)

	// TODO: Add logic for update
	return ctrl.Result{}, nil
}

// Handle Delete Event
func (r *KuberhealthyJobReconciler) handleDelete(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Handling Delete Event", "name", req.NamespacedName)

	// TODO: Add cleanup logic
	return ctrl.Result{}, nil
}
