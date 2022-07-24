package reconcilers

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/opensearch-gateway/requests"
	"opensearch.opster.io/opensearch-gateway/services"
	"opensearch.opster.io/pkg/helpers"
	"opensearch.opster.io/pkg/reconcilers/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type UserRoleBindingReconciler struct {
	client.Client
	ReconcilerOptions
	ctx      context.Context
	osClient *services.OsClusterClient
	recorder record.EventRecorder
	instance *opsterv1.OpensearchUserRoleBinding
	cluster  *opsterv1.OpenSearchCluster
	logger   logr.Logger
}

func NewUserRoleBindingReconciler(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	instance *opsterv1.OpensearchUserRoleBinding,
	opts ...ReconcilerOption,
) *UserRoleBindingReconciler {
	options := ReconcilerOptions{}
	options.apply(opts...)
	return &UserRoleBindingReconciler{
		Client:            client,
		ReconcilerOptions: options,
		ctx:               ctx,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx).WithValues("reconciler", "user"),
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
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(r.ctx, client.ObjectKeyFromObject(r.instance), r.instance); err != nil {
				return err
			}
			r.instance.Status.Reason = reason
			if retErr != nil {
				r.instance.Status.State = opsterv1.OpensearchUserRoleBindingStateError
			}
			if retResult.Requeue {
				r.instance.Status.State = opsterv1.OpensearchUserRoleBindingPending
			}
			if retErr == nil && !retResult.Requeue {
				r.instance.Status.ProvisionedRoles = r.instance.Spec.Roles
				r.instance.Status.ProvisionedUsers = r.instance.Spec.Users
				r.instance.Status.State = opsterv1.OpensearchUserRoleBindingStateCreated
			}
			return r.Status().Update(r.ctx, r.instance)
		})

		if err != nil {
			r.logger.Error(err, "failed to update status")
		}
	}()

	r.cluster, retErr = util.FetchOpensearchCluster(r.ctx, r.Client, r.instance.Spec.OpensearchRef)
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
		if !reflect.DeepEqual(*r.instance.Status.ManagedCluster, r.instance.Spec.OpensearchRef) {
			reason = "cannot change the cluster a userrolebinding refers to"
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
				r.instance.Status.ManagedCluster = &r.instance.Spec.OpensearchRef
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
			retErr = r.removeUsersFromMapping(removed, r.instance.Status.ProvisionedUsers)
			if retErr != nil {
				reason = "failed to update existing role mapping"
				r.logger.Error(retErr, reason)
				r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
				return
			}
		}
	}

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
			// First remove any users that are no longer in the spec
			removedUsers := r.calculateRemovedUsers()
			if len(removedUsers) > 0 {
				retErr = r.removeUsersFromMapping(role, removedUsers)
				if retErr != nil {
					reason = "failed to update existing role mapping"
					r.logger.Error(retErr, reason)
					r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
					return
				}
			}
			// Then add new users
			retErr = r.reconcileExistingMapping(role)
			if retErr != nil {
				reason = "failed to update existing role mapping"
				r.logger.Error(retErr, reason)
				r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
				return
			}
			continue
		}

		mapping := requests.RoleMapping{
			Users: r.instance.Spec.Users,
		}
		retErr = services.CreateOrUpdateRoleMapping(r.ctx, r.osClient, role, mapping)
		if retErr != nil {
			reason = "failed to create role mapping"
			r.logger.Error(retErr, reason)
			r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
			return
		}
	}

	return
}

func (r *UserRoleBindingReconciler) Delete() error {
	var err error
	r.cluster, err = util.FetchOpensearchCluster(r.ctx, r.Client, r.instance.Spec.OpensearchRef)
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

	for _, role := range r.instance.Status.ProvisionedRoles {
		exist, err := services.RoleMappingExists(r.ctx, r.osClient, role)
		if err != nil {
			return err
		}
		if !exist {
			r.logger.V(1).Info("role mapping already deleted from opensearch")
			continue
		}
		err = r.removeUsersFromMapping(role, r.instance.Status.ProvisionedUsers)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *UserRoleBindingReconciler) reconcileExistingMapping(rolename string) error {
	mapping, err := services.FetchExistingRoleMapping(r.ctx, r.osClient, rolename)
	if err != nil {
		return err
	}

	newUser := false
	for _, user := range r.instance.Spec.Users {
		if !helpers.ContainsString(mapping.Users, user) {
			mapping.Users = append(mapping.Users, user)
			newUser = true
		}
	}

	if !newUser {
		return nil
	}

	return services.CreateOrUpdateRoleMapping(r.ctx, r.osClient, rolename, mapping)
}

func (r *UserRoleBindingReconciler) removeUsersFromMapping(rolename string, usersToRemove []string) error {
	users := []string{}
	mapping, err := services.FetchExistingRoleMapping(r.ctx, r.osClient, rolename)
	if err != nil {
		return err
	}

	for _, user := range mapping.Users {
		if !helpers.ContainsString(usersToRemove, user) {
			users = append(users, user)
		}
	}

	if len(users) == len(mapping.Users) && len(users) > 0 {
		return nil
	}

	mapping.Users = users

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

func (r *UserRoleBindingReconciler) calculateRemovedUsers() []string {
	var usersRemoved []string
	for _, user := range r.instance.Status.ProvisionedUsers {
		if !helpers.ContainsString(r.instance.Spec.Users, user) {
			usersRemoved = append(usersRemoved, user)
		}
	}

	return usersRemoved
}
