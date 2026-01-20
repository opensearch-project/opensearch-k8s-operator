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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers"
)

// OpensearchUserReconciler reconciles a OpensearchUser object
type OpensearchUserReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Instance *opensearchv1.OpensearchUser
	logr.Logger
}

//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchusers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchusers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchusers/finalizers,verbs=update
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchusers,verbs=get;list;watch
//+kubebuilder:rbac:groups=opensearch.org,resources=opensearchclusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *OpensearchUserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Logger = log.FromContext(ctx).WithValues("user", req.NamespacedName)
	r.Logger.V(4).Info("Reconciling OpensearchUser")

	r.Instance = &opensearchv1.OpensearchUser{}
	err := r.Get(ctx, req.NamespacedName, r.Instance)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	userReconciler := reconcilers.NewUserReconciler(
		r.Client,
		ctx,
		r.Recorder,
		r.Instance,
	)

	if r.Instance.DeletionTimestamp.IsZero() {
		controllerutil.AddFinalizer(r.Instance, OpensearchFinalizer)
		err = r.Update(ctx, r.Instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		return userReconciler.Reconcile()
	} else {
		if controllerutil.ContainsFinalizer(r.Instance, OpensearchFinalizer) {
			err = userReconciler.Delete()
			if err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(r.Instance, OpensearchFinalizer)
			return ctrl.Result{}, r.Update(ctx, r.Instance)
		}
	}

	return ctrl.Result{}, nil
}

func (r *OpensearchUserReconciler) handleSecretEvent(_ context.Context, secret client.Object) []reconcile.Request {
	var reconcileRequests []reconcile.Request

	if secret == nil {
		return reconcileRequests
	}

	// Only check secrets with OsUserNamespaceAnnotation and (optional) OsUserNameAnnotation
	annotations := secret.GetAnnotations()

	namespace, namespaceOk := annotations[helpers.OsUserNamespaceAnnotation]

	if !namespaceOk {
		return reconcileRequests
	}

	name, nameOk := annotations[helpers.OsUserNameAnnotation]

	if nameOk {
		return append(reconcileRequests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			},
		})
	}

	// For Secret that stores multiple User passwords
	// Cast the client.Object to a *corev1.Secret
	secretObj, ok := secret.(*corev1.Secret)
	if !ok {
		return reconcileRequests
	}

	for username := range secretObj.Data {
		// Create a reconcile request for each user found in the Secret
		reconcileRequests = append(reconcileRequests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      username,
				Namespace: namespace,
			},
		})
	}

	return reconcileRequests
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpensearchUserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opensearchv1.OpensearchUser{}).
		// Get notified when opensearch clusters change
		Owns(&opensearchv1.OpenSearchCluster{}).
		// Get notified when password backing secret changes
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.handleSecretEvent),
		).
		Complete(r)
}
