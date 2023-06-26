package reconcilers

import (
	"context"
	"fmt"
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
	"time"
)

const (
	opensearchActionGroupExists = "actiongroup already exists in Opensearch; not modifying"
)

type ActionGroupReconciler struct {
	client.Client
	ReconcilerOptions
	ctx      context.Context
	osClient *services.OsClusterClient
	recorder record.EventRecorder
	instance *opsterv1.OpensearchActionGroup
	cluster  *opsterv1.OpenSearchCluster
	logger   logr.Logger
}

func NewActionGroupReconciler(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	instance *opsterv1.OpensearchActionGroup,
	opts ...ReconcilerOption,
) *ActionGroupReconciler {
	options := ReconcilerOptions{}
	options.apply(opts...)
	return &ActionGroupReconciler{
		Client:            client,
		ReconcilerOptions: options,
		ctx:               ctx,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx).WithValues("reconciler", "actiongroup"),
	}
}

func (r *ActionGroupReconciler) Reconcile() (retResult ctrl.Result, retErr error) {
	var reason string

	defer func() {
		if !pointer.BoolDeref(r.updateStatus, true) {
			return
		}
		// When the reconciler is done, figure out what the state of the resource
		// is and set it in the state field accordingly.
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(r.ctx, client.ObjectKeyFromObject(r.instance), r.instance); err != nil {
				return err
			}
			r.instance.Status.Reason = reason
			if retErr != nil {
				r.instance.Status.State = opsterv1.OpensearchActionGroupError
			}
			if retResult.Requeue && retResult.RequeueAfter == 10*time.Second {
				r.instance.Status.State = opsterv1.OpensearchActionGroupPending
			}
			if retErr == nil && retResult.RequeueAfter == 30*time.Second {
				r.instance.Status.State = opsterv1.OpensearchActionGroupCreated
			}
			if reason == opensearchActionGroupExists {
				r.instance.Status.State = opsterv1.OpensearchActionGroupIgnored
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
			reason = "cannot change the cluster an actiongroup refers to"
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

	// Check actiongroup state to make sure we don't touch preexisting actiongroups
	if r.instance.Status.ExistingActionGroup == nil {
		var exists bool
		exists, retErr = services.ActionGroupExists(r.ctx, r.osClient, r.instance.Name)
		if retErr != nil {
			reason = "failed to get actiongroup status from Opensearch API"
			r.logger.Error(retErr, reason)
			r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
			return
		}
		if pointer.BoolDeref(r.updateStatus, true) {
			retErr = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := r.Get(r.ctx, client.ObjectKeyFromObject(r.instance), r.instance); err != nil {
					return err
				}
				r.instance.Status.ExistingActionGroup = &exists
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

	// If actiongroup is existing do nothing
	if *r.instance.Status.ExistingActionGroup {
		reason = opensearchActionGroupExists
		return
	}

	actionGroup := requests.ActionGroup{
		AllowedActions: r.instance.Spec.AllowedActions,
	}

	if len(r.instance.Spec.Type) > 0 {
		actionGroup.Type = r.instance.Spec.Type
	}

	if len(r.instance.Spec.Description) > 0 {
		actionGroup.Description = r.instance.Spec.Description
	}

	shouldUpdate, retErr := services.ShouldUpdateActionGroup(r.ctx, r.osClient, r.instance.Name, actionGroup)
	if retErr != nil {
		reason = "failed to get actiongroup status from Opensearch API"
		r.logger.Error(retErr, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
		return
	}

	if !shouldUpdate {
		r.logger.V(1).Info(fmt.Sprintf("actiongroup %s is in sync", r.instance.Name))
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, retErr
	}

	retErr = services.CreateOrUpdateActionGroup(r.ctx, r.osClient, r.instance.Name, actionGroup)
	if retErr != nil {
		reason = "failed to update actiongroup with Opensearch API"
		r.logger.Error(retErr, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
	}

	r.recorder.Event(r.instance, "Normal", opensearchAPIUpdated, "actiongroup updated in opensearch")

	return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, retErr
}

func (r *ActionGroupReconciler) Delete() error {
	// If we have never successfully reconciled we can just exit
	if r.instance.Status.ExistingActionGroup == nil {
		return nil
	}

	if *r.instance.Status.ExistingActionGroup {
		r.logger.Info("actiongroup was pre-existing; not deleting")
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

	exist, err := services.ActionGroupExists(r.ctx, r.osClient, r.instance.Name)
	if err != nil {
		return err
	}
	if !exist {
		r.logger.V(1).Info("actiongroup already deleted from opensearch")
		return nil
	}

	return services.DeleteActionGroup(r.ctx, r.osClient, r.instance.Name)
}
