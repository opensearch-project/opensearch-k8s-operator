package reconcilers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/opensearch-gateway/requests"
	"opensearch.opster.io/opensearch-gateway/services"
	"opensearch.opster.io/pkg/reconcilers/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	opensearchRoleExists = "role already exists in Opensearch; not modifying"
)

type RoleReconciler struct {
	client.Client
	ReconcilerOptions
	ctx      context.Context
	osClient *services.OsClusterClient
	recorder record.EventRecorder
	instance *opsterv1.OpensearchRole
	cluster  *opsterv1.OpenSearchCluster
	logger   logr.Logger
}

func NewRoleReconciler(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	instance *opsterv1.OpensearchRole,
	opts ...ReconcilerOption,
) *RoleReconciler {
	options := ReconcilerOptions{}
	options.apply(opts...)
	return &RoleReconciler{
		Client:            client,
		ReconcilerOptions: options,
		ctx:               ctx,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx).WithValues("reconciler", "role"),
	}
}

func (r *RoleReconciler) Reconcile() (retResult ctrl.Result, retErr error) {
	var reason string

	defer func() {
		if !pointer.BoolDeref(r.updateStatus, true) {
			return
		}
		// When the reconciler is done, figure out what the state of the resource is
		// is and set it in the state field accordingly.
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(r.ctx, client.ObjectKeyFromObject(r.instance), r.instance); err != nil {
				return err
			}
			r.instance.Status.Reason = reason
			if retErr != nil {
				r.instance.Status.State = opsterv1.OpensearchRoleStateError
			}
			if retResult.Requeue && retResult.RequeueAfter == 10*time.Second {
				r.instance.Status.State = opsterv1.OpensearchRoleStatePending
			}
			if retErr == nil && retResult.Requeue {
				r.instance.Status.State = opsterv1.OpensearchRoleStateCreated
			}
			if reason == opensearchRoleExists {
				r.instance.Status.State = opsterv1.OpensearchRoleIgnored
			}
			return r.Status().Update(r.ctx, r.instance)
		})

		if err != nil {
			r.logger.Error(err, "failed to update status")
		}
	}()

	r.cluster, retErr = util.FetchOpensearchCluster(r.ctx, r.Client, types.NamespacedName{
		Name:      r.instance.Spec.OpensearchRef.Name,
		Namespace: r.instance.Namespace,
	})
	if retErr != nil {
		reason = "error fetching opensearch cluster"
		r.logger.Error(retErr, "failed to fetch opensearch cluster")
		r.recorder.Event(r.instance, "Warning", opensearchError, reason)
		return
	}
	if r.cluster == nil {
		r.logger.Info("opensearch cluster does not exist, requeueing")
		reason = "waiting for opensearch cluster to exist"
		r.recorder.Event(r.instance, "Normal", opensearchPending, reason)
		retResult = ctrl.Result{
			Requeue:      true,
			RequeueAfter: 10 * time.Second,
		}
		return
	}

	// Check cluster ref has not changed
	if r.instance.Status.ManagedCluster != nil {
		if *r.instance.Status.ManagedCluster != r.cluster.UID {
			reason = "cannot change the cluster a role refers to"
			retErr = fmt.Errorf("%s", reason)
			r.recorder.Event(r.instance, "Warning", opensearchRefMismatch, reason)
			return
		}
	} else {
		if pointer.BoolDeref(r.updateStatus, true) {
			retErr = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := r.Get(r.ctx, client.ObjectKeyFromObject(r.instance), r.instance); err != nil {
					return err
				}
				r.instance.Status.ManagedCluster = &r.cluster.UID
				return r.Status().Update(r.ctx, r.instance)
			})
			if retErr != nil {
				reason = fmt.Sprintf("failed to update status: %s", retErr)
				r.recorder.Event(r.instance, "Warning", statusError, reason)
				return
			}
		}
	}

	// Check cluster is ready
	if r.cluster.Status.Phase != opsterv1.PhaseRunning {
		r.logger.Info("opensearch cluster is not running, requeueing")
		reason = "waiting for opensearch cluster status to be running"
		r.recorder.Event(r.instance, "Normal", opensearchPending, reason)
		retResult = ctrl.Result{
			Requeue:      true,
			RequeueAfter: 10 * time.Second,
		}
		return
	}

	r.osClient, retErr = util.CreateClientForCluster(r.ctx, r.Client, r.cluster, r.osClientTransport)
	if retErr != nil {
		reason = "error creating opensearch client"
		r.recorder.Event(r.instance, "Warning", opensearchError, reason)
		return
	}

	// Check role state to make sure we don't touch preexisting roles
	if r.instance.Status.ExistingRole == nil {
		var exists bool
		exists, retErr = services.RoleExists(r.ctx, r.osClient, r.instance.Name)
		if retErr != nil {
			reason = "failed to get user status from Opensearch API"
			r.logger.Error(retErr, reason)
			r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
			return
		}
		if pointer.BoolDeref(r.updateStatus, true) {
			retErr = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := r.Get(r.ctx, client.ObjectKeyFromObject(r.instance), r.instance); err != nil {
					return err
				}
				r.instance.Status.ExistingRole = &exists
				return r.Status().Update(r.ctx, r.instance)
			})
			if retErr != nil {
				reason = fmt.Sprintf("failed to update status: %s", retErr)
				r.recorder.Event(r.instance, "Warning", statusError, reason)
				return
			}
		} else {
			// Emit an event for unit testing assertion
			r.recorder.Event(r.instance, "Normal", "UnitTest", fmt.Sprintf("exists is %t", exists))
			return
		}
	}

	// If role is existing do nothing
	if *r.instance.Status.ExistingRole {
		reason = opensearchRoleExists
		return
	}

	role := requests.Role{
		ClusterPermissions: r.instance.Spec.ClusterPermissions,
	}

	if len(r.instance.Spec.IndexPermissions) > 0 {
		role.IndexPermissions = make([]requests.IndexPermissionSpec, 0, len(r.instance.Spec.IndexPermissions))
		for _, permission := range r.instance.Spec.IndexPermissions {
			role.IndexPermissions = append(role.IndexPermissions, requests.IndexPermissionSpec(permission))
		}
	}

	if len(r.instance.Spec.TenantPermissions) > 0 {
		role.TenantPermissions = make([]requests.TenantPermissionsSpec, 0, len(r.instance.Spec.TenantPermissions))
		for _, permission := range r.instance.Spec.TenantPermissions {
			role.TenantPermissions = append(role.TenantPermissions, requests.TenantPermissionsSpec(permission))
		}
	}

	shouldUpdate, retErr := services.ShouldUpdateRole(r.ctx, r.osClient, r.instance.Name, role)
	if retErr != nil {
		reason = "failed to get role status from Opensearch API"
		r.logger.Error(retErr, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
		return
	}

	if !shouldUpdate {
		r.logger.V(1).Info(fmt.Sprintf("role %s is in sync", r.instance.Name))
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, retErr
	}

	retErr = services.CreateOrUpdateRole(r.ctx, r.osClient, r.instance.Name, role)
	if retErr != nil {
		reason = "failed to update role with Opensearch API"
		r.logger.Error(retErr, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
	}

	r.recorder.Event(r.instance, "Normal", opensearchAPIUpdated, "role updated in opensearch")

	return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, retErr
}

func (r *RoleReconciler) Delete() error {
	// If we have never successfully reconciled we can just exit
	if r.instance.Status.ExistingRole == nil {
		return nil
	}

	if *r.instance.Status.ExistingRole {
		r.logger.Info("role was pre-existing; not deleting")
		return nil
	}

	var err error

	r.cluster, err = util.FetchOpensearchCluster(r.ctx, r.Client, types.NamespacedName{
		Name:      r.instance.Spec.OpensearchRef.Name,
		Namespace: r.instance.Namespace,
	})
	if err != nil {
		return err
	}

	if r.cluster == nil || !r.cluster.DeletionTimestamp.IsZero() {
		// If the opensearch cluster doesn't exist, we don't need to delete anything
		return nil
	}

	r.osClient, err = util.CreateClientForCluster(r.ctx, r.Client, r.cluster, r.osClientTransport)
	if err != nil {
		return err
	}

	exist, err := services.RoleExists(r.ctx, r.osClient, r.instance.Name)
	if err != nil {
		return err
	}
	if !exist {
		r.logger.V(1).Info("role already deleted from opensearch")
		return nil
	}

	return services.DeleteRole(r.ctx, r.osClient, r.instance.Name)
}
