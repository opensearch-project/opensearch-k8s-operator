/*
Copyright 2021.

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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/reconcilers"
)

// OpensearchRoleReconciler reconciles a OpensearchRole object
type OpensearchRoleReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Instance *opsterv1.OpensearchRole
	logr.Logger
}

//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchroles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchroles/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *OpensearchRoleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Logger = log.FromContext(ctx).WithValues("role", req.NamespacedName)
	r.Logger.Info("Reconciling OpensearchRole")

	r.Instance = &opsterv1.OpensearchRole{}
	err := r.Get(ctx, req.NamespacedName, r.Instance)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	roleReconciler := reconcilers.NewRoleReconciler(
		ctx,
		r.Client,
		r.Recorder,
		r.Instance,
	)

	if r.Instance.DeletionTimestamp.IsZero() {
		controllerutil.AddFinalizer(r.Instance, OpensearchFinalizer)
		err = r.Client.Update(ctx, r.Instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		return roleReconciler.Reconcile()
	} else {
		if controllerutil.ContainsFinalizer(r.Instance, OpensearchFinalizer) {
			err = roleReconciler.Delete()
			if err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(r.Instance, OpensearchFinalizer)
			return ctrl.Result{}, r.Client.Update(ctx, r.Instance)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpensearchRoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsterv1.OpensearchRole{}).
		Owns(&opsterv1.OpenSearchCluster{}). // Get notified when opensearch clusters change
		Complete(r)
}
