package reconcilers

import (
	"context"
	"fmt"
	"time"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	"github.com/cisco-open/operator-tools/pkg/reconciler"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type UserRoleBindingReconciler struct {
	client k8s.K8sClient
	ReconcilerOptions
	ctx      context.Context
	osClient *services.OsClusterClient
	recorder record.EventRecorder
	instance *opsterv1.OpensearchUserRoleBinding
	cluster  *opsterv1.OpenSearchCluster
	logger   logr.Logger
}

func NewUserRoleBindingReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	instance *opsterv1.OpensearchUserRoleBinding,
	opts ...ReconcilerOption,
) *UserRoleBindingReconciler {
	options := ReconcilerOptions{}
	options.apply(opts...)
	return &UserRoleBindingReconciler{
		client:            k8s.NewK8sClient(client, ctx, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "userrolebinding"))),
		ReconcilerOptions: options,
		ctx:               ctx,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx).WithValues("reconciler", "userrolebinding"),
	}
}

func (r *UserRoleBindingReconciler) Reconcile() (retResult ctrl.Result, retErr error) {
	var reason string

	defer func() {
		// Skip status updates when option is set
		if !pointer.BoolDeref(r.updateStatus, true) {
			return
		}
		// When the reconciler is done, figure out what the state of the resource is
		// is and set it in the state field accordingly.
		err := r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			instance := object.(*opsterv1.OpensearchUserRoleBinding)
			instance.Status.Reason = reason
			if retErr != nil {
				instance.Status.State = opsterv1.OpensearchUserRoleBindingStateError
			}
			if retResult.Requeue && retResult.RequeueAfter == 10*time.Second {
				instance.Status.State = opsterv1.OpensearchUserRoleBindingPending
			}
			if retErr == nil && retResult.RequeueAfter == 30*time.Second {
				instance.Status.ProvisionedRoles = instance.Spec.Roles
				instance.Status.ProvisionedBackendRoles = instance.Spec.BackendRoles
				instance.Status.ProvisionedUsers = instance.Spec.Users
				instance.Status.State = opsterv1.OpensearchUserRoleBindingStateCreated
			}
		})
		if err != nil {
			r.logger.Error(err, "failed to update status")
		}
	}()

	r.cluster, retErr = util.FetchOpensearchCluster(r.client, r.ctx, types.NamespacedName{
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
			reason = "cannot change the cluster a userrolebinding refers to"
			retErr = fmt.Errorf("%s", reason)
			r.recorder.Event(r.instance, "Warning", opensearchRefMismatch, reason)
			return
		}
	} else {
		if pointer.BoolDeref(r.updateStatus, true) {
			retErr = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
				instance := object.(*opsterv1.OpensearchUserRoleBinding)
				instance.Status.ManagedCluster = &r.cluster.UID
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

	r.osClient, retErr = util.CreateClientForCluster(r.client, r.ctx, r.cluster, r.osClientTransport)
	if retErr != nil {
		reason = "error creating opensearch client"
		r.recorder.Event(r.instance, "Warning", opensearchError, reason)
		return
	}

	// Reconcile any roles that have been removed
	rolesRemoved := r.calculateRemovedRoles()
	for _, removed := range rolesRemoved {
		var exists bool
		exists, retErr = services.RoleMappingExists(r.ctx, r.osClient, removed)
		if retErr != nil {
			reason = "failed to get role mapping status from Opensearch API"
			r.logger.Error(retErr, reason)
			r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
			return
		}
		if exists {
			retErr = r.removeObjectsFromMapping(removed, r.instance.Status.ProvisionedUsers, r.instance.Status.ProvisionedBackendRoles)
			if retErr != nil {
				reason = "failed to update existing role mapping"
				r.logger.Error(retErr, reason)
				r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
				return
			}
		}
	}

	// Reconcile roles
	for _, role := range r.instance.Spec.Roles {
		var exists bool
		exists, retErr = services.RoleMappingExists(r.ctx, r.osClient, role)
		if retErr != nil {
			reason = "failed to get role mapping status from Opensearch API"
			r.logger.Error(retErr, reason)
			r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
			return
		}

		if exists {
			// Replace existing mapping with new one
			removedUsers := helpers.DiffSlice(r.instance.Status.ProvisionedUsers, r.instance.Spec.Users)
			removedBackendRoles := helpers.DiffSlice(r.instance.Status.ProvisionedBackendRoles, r.instance.Spec.BackendRoles)
			retErr = r.reconcileExistingMapping(role, removedUsers, removedBackendRoles)
			if retErr != nil {
				reason = "failed to update existing role mapping"
				r.logger.Error(retErr, reason)
				r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
				return
			}
			continue
		}

		mapping := requests.RoleMapping{
			Users:        r.instance.Spec.Users,
			BackendRoles: r.instance.Spec.BackendRoles,
		}
		retErr = services.CreateOrUpdateRoleMapping(r.ctx, r.osClient, role, mapping)
		if retErr != nil {
			reason = "failed to create role mapping"
			r.logger.Error(retErr, reason)
			r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
			return
		}
	}

	return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, retErr
}

func (r *UserRoleBindingReconciler) Delete() error {
	var err error
	r.cluster, err = util.FetchOpensearchCluster(r.client, r.ctx, types.NamespacedName{
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

	r.osClient, err = util.CreateClientForCluster(r.client, r.ctx, r.cluster, r.osClientTransport)
	if err != nil {
		return err
	}

	for _, role := range r.instance.Status.ProvisionedRoles {
		exist, err := services.RoleMappingExists(r.ctx, r.osClient, role)
		if err != nil {
			return err
		}
		if !exist {
			r.logger.V(1).Info("role mapping already deleted from opensearch")
			continue
		}
		err = r.removeObjectsFromMapping(role, r.instance.Status.ProvisionedUsers, r.instance.Status.ProvisionedBackendRoles)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *UserRoleBindingReconciler) reconcileExistingMapping(rolename string, usersToRemove, backendRolesToRemove []string) error {
	mapping, err := services.FetchExistingRoleMapping(r.ctx, r.osClient, rolename)
	if err != nil {
		return err
	}

	usersToSave := helpers.DiffSlice(mapping.Users, usersToRemove)
	backendRolesToSave := helpers.DiffSlice(mapping.BackendRoles, backendRolesToRemove)
	newUsers := helpers.DiffSlice(r.instance.Spec.Users, mapping.Users)
	newBackendRoles := helpers.DiffSlice(r.instance.Spec.BackendRoles, mapping.BackendRoles)

	if newUsers == nil && newBackendRoles == nil && len(usersToSave) == len(mapping.Users) && len(backendRolesToSave) == len(mapping.BackendRoles) {
		return nil
	}
	mapping.Users = append(usersToSave, newUsers...)
	mapping.BackendRoles = append(backendRolesToSave, newBackendRoles...)

	if len(mapping.Users) > 0 || len(mapping.Hosts) > 0 || len(mapping.BackendRoles) > 0 {
		return services.CreateOrUpdateRoleMapping(r.ctx, r.osClient, rolename, mapping)
	}
	return services.DeleteRoleMapping(r.ctx, r.osClient, rolename)
}

func (r *UserRoleBindingReconciler) removeObjectsFromMapping(rolename string, usersToRemove, backendRolesToRemove []string) error {
	mapping, err := services.FetchExistingRoleMapping(r.ctx, r.osClient, rolename)
	if err != nil {
		return err
	}

	usersToSave := helpers.DiffSlice(mapping.Users, usersToRemove)
	backendRolesToSave := helpers.DiffSlice(mapping.BackendRoles, backendRolesToRemove)

	if len(usersToSave) == len(mapping.Users) && len(usersToSave) > 0 && len(backendRolesToSave) == len(mapping.BackendRoles) && len(backendRolesToSave) > 0 {
		return nil
	}

	mapping.Users = usersToSave
	mapping.BackendRoles = backendRolesToSave

	if len(mapping.Users) > 0 || len(mapping.Hosts) > 0 || len(mapping.BackendRoles) > 0 {
		return services.CreateOrUpdateRoleMapping(r.ctx, r.osClient, rolename, mapping)
	}

	return services.DeleteRoleMapping(r.ctx, r.osClient, rolename)
}

func (r *UserRoleBindingReconciler) calculateRemovedRoles() []string {
	var rolesRemoved []string
	for _, role := range r.instance.Status.ProvisionedRoles {
		if !helpers.ContainsString(r.instance.Spec.Roles, role) {
			rolesRemoved = append(rolesRemoved, role)
		}
	}

	return rolesRemoved
}
