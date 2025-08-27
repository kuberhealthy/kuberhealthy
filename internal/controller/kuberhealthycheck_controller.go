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
	"encoding/json"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	khcrdsv2 "github.com/kuberhealthy/crds/api/v2"
	kuberhealthy "github.com/kuberhealthy/kuberhealthy/v3/internal/kuberhealthy"
)

// KuberhealthyCheckReconciler reconciles KuberhealthyCheck resources
type KuberhealthyCheckReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Kuberhealthy *kuberhealthy.Kuberhealthy
}

// +kubebuilder:rbac:groups=kuberhealthy.github.io,resources=kuberhealthychecks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kuberhealthy.github.io,resources=kuberhealthychecks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kuberhealthy.github.io,resources=kuberhealthychecks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KuberhealthyCheck object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
func (r *KuberhealthyCheckReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Debugln("controller: Reconcile")
	var check khcrdsv2.KuberhealthyCheck
	if err := r.Get(ctx, req.NamespacedName, &check); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	finalizer := "kuberhealthy.github.io/finalizer"

	// DELETE support for finalizer
	if !check.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&check, finalizer) {
			log.Infoln("controller: FINALIZER DELETE event detected for:", req.Namespace+"/"+req.Name)

			// Remove finalizer and update the resource
			controllerutil.RemoveFinalizer(&check, finalizer)
			logPodSpecObjectMeta("delete-finalizer update", &check)
			if err := r.Update(ctx, &check); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer is set
	if !controllerutil.ContainsFinalizer(&check, finalizer) {
		controllerutil.AddFinalizer(&check, finalizer)
		logPodSpecObjectMeta("add-finalizer update", &check)
		if err := r.Update(ctx, &check); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Optionally update status of the kuberhealthycheck resource
	// check.Status.Phase = "Running"
	logPodSpecObjectMeta("status update", &check)
	if err := r.Status().Update(ctx, &check); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// logPodSpecObjectMeta prints the pod template metadata to help trace unknown field warnings.
func logPodSpecObjectMeta(stage string, check *khcrdsv2.KuberhealthyCheck) {
	metaBytes, err := json.Marshal(check.Spec.PodSpec.ObjectMeta)
	if err != nil {
		// Log the marshaling error for additional context.
		log.WithFields(log.Fields{"stage": stage, "error": err}).Debug("marshal podSpec metadata failed")
		return
	}

	ct := check.Spec.PodSpec.ObjectMeta.CreationTimestamp
	log.WithFields(log.Fields{
		"stage":             stage,
		"creationTimestamp": ct,
		"metadata":          string(metaBytes),
	}).Debug("podSpec metadata contents")
}
