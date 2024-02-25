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

type UserReconciler struct {
	client k8s.K8sClient
	ReconcilerOptions
	ctx      context.Context
	osClient *services.OsClusterClient
	recorder record.EventRecorder
	instance *opsterv1.OpensearchUser
	cluster  *opsterv1.OpenSearchCluster
	logger   logr.Logger
}

func NewUserReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	instance *opsterv1.OpensearchUser,
	opts ...ReconcilerOption,
) *UserReconciler {
	options := ReconcilerOptions{}
	options.apply(opts...)
	return &UserReconciler{
		client:            k8s.NewK8sClient(client, ctx, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "user"))),
		ReconcilerOptions: options,
		ctx:               ctx,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx).WithValues("reconciler", "user"),
	}
}

func (r *UserReconciler) Reconcile() (retResult ctrl.Result, retErr error) {
	var reason string

	defer func() {
		// Skip status updates when option is set
		if !pointer.BoolDeref(r.updateStatus, true) {
			return
		}
		// When the reconciler is done, figure out what the state of the resource is
		// is and set it in the state field accordingly.
		err := r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			instance := object.(*opsterv1.OpensearchUser)
			instance.Status.Reason = reason
			if retErr != nil {
				instance.Status.State = opsterv1.OpensearchUserStateError
			}
			if retResult.Requeue && retResult.RequeueAfter == 10*time.Second {
				instance.Status.State = opsterv1.OpensearchUserStatePending
			}
			if retErr == nil && retResult.RequeueAfter == 30*time.Second {
				instance.Status.State = opsterv1.OpensearchUserStateCreated
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
			reason = "cannot change the cluster a user refers to"
			retErr = fmt.Errorf("%s", reason)
			r.recorder.Event(r.instance, "Warning", opensearchRefMismatch, reason)
			return
		}
	} else {
		if pointer.BoolDeref(r.updateStatus, true) {
			retErr = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
				instance := object.(*opsterv1.OpensearchUser)
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

	userPassword, retErr := r.managePasswordSecret(r.instance.Name, r.instance.Namespace)

	if retErr != nil {
		// Event and logging handled in fetch function
		reason = "failed to get password from secret"
		return
	}
	user := requests.User{
		Password:                userPassword,
		OpendistroSecurityRoles: r.instance.Spec.OpendistroSecurityRoles,
		BackendRoles:            r.instance.Spec.BackendRoles,
		Attributes:              r.instance.Spec.Attributes,
	}

	// Instantiate the map first
	if user.Attributes == nil {
		user.Attributes = map[string]string{
			services.K8sAttributeField: string(r.instance.GetUID()),
		}
	} else {
		user.Attributes[services.K8sAttributeField] = string(r.instance.GetUID())
	}

	update, retErr := services.ShouldUpdateUser(r.ctx, r.osClient, r.instance.Name, user)
	if retErr != nil {
		reason = "failed to get user status from Opensearch API"
		r.logger.Error(retErr, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
		return
	}
	if !update {
		r.logger.V(1).Info(fmt.Sprintf("user %s is in sync", r.instance.Name))
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, retErr
	}

	retErr = services.CreateOrUpdateUser(r.ctx, r.osClient, r.instance.Name, user)
	if retErr != nil {
		reason = "failed to get update user with Opensearch API"
		r.logger.Error(retErr, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
	}

	r.recorder.Event(r.instance, "Normal", opensearchAPIUpdated, "user updated in opensearch")
	return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, retErr
}

func (r *UserReconciler) Delete() error {
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

	exist, err := services.UserExists(r.ctx, r.osClient, r.instance.Name)
	if err != nil {
		return err
	}
	if !exist {
		r.logger.V(1).Info("user already deleted from opensearch")
		return nil
	}

	matches, err := services.UserUIDMatches(r.ctx, r.osClient, r.instance.Name, string(r.instance.GetUID()))
	if err != nil {
		return err
	}
	if !matches {
		r.logger.V(1).Error(
			fmt.Errorf("UID doesn't match user in Opensearch"),
			"UID mismatch, deleting kubernetes object but not user in Opensearch",
		)
		return nil
	}

	return services.DeleteUser(r.ctx, r.osClient, r.instance.Name)
}

func (r *UserReconciler) managePasswordSecret(username string, namespace string) (string, error) {
	secret, err := r.client.GetSecret(r.instance.Spec.PasswordFrom.Name, r.instance.Namespace)
	if err != nil {
		r.logger.V(1).Error(err, "failed to fetch password secret")
		r.recorder.Event(r.instance, "Warning", passwordError, "error fetching password secret")
		return "", err
	}

	// Patch OpenSearch Annotations onto secret
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}

	secret.Annotations[helpers.OsUserNameAnnotation] = username
	secret.Annotations[helpers.OsUserNamespaceAnnotation] = namespace

	if _, err := r.client.CreateSecret(&secret); err != nil {
		r.logger.V(1).Error(err, "failed to patch opensearch username onto password secret")
		r.recorder.Event(r.instance, "Warning", passwordError, "error patching opensearch username onto password secret")
		return "", err
	}

	userPassword, ok := secret.Data[r.instance.Spec.PasswordFrom.Key]
	if !ok {
		err = fmt.Errorf("key %s does not exist in secret", r.instance.Spec.PasswordFrom.Key)
		r.logger.V(1).Error(err, "failed to get password from secret")
		r.recorder.Event(r.instance, "Warning", passwordError, fmt.Sprintf("key %s does not exist in secret", r.instance.Spec.PasswordFrom.Key))
		return "", err
	}

	return string(userPassword), nil
}
