package reconcilers

import (
	"context"
	"fmt"
	"time"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
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

const (
	opensearchTenantExists = "tenant already exists in Opensearch; not modifying"
)

type TenantReconciler struct {
	client k8s.K8sClient
	ReconcilerOptions
	ctx      context.Context
	osClient *services.OsClusterClient
	recorder record.EventRecorder
	instance *opsterv1.OpensearchTenant
	cluster  *opsterv1.OpenSearchCluster
	logger   logr.Logger
}

func NewTenantReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	instance *opsterv1.OpensearchTenant,
	opts ...ReconcilerOption,
) *TenantReconciler {
	options := ReconcilerOptions{}
	options.apply(opts...)
	return &TenantReconciler{
		client:            k8s.NewK8sClient(client, ctx, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "tenant"))),
		ReconcilerOptions: options,
		ctx:               ctx,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx).WithValues("reconciler", "tenant"),
	}
}

func (r *TenantReconciler) Reconcile() (retResult ctrl.Result, retErr error) {
	var reason string

	defer func() {
		if !pointer.BoolDeref(r.updateStatus, true) {
			return
		}
		// When the reconciler is done, figure out what the state of the resource
		// is and set it in the state field accordingly.
		err := r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			instance := object.(*opsterv1.OpensearchTenant)
			instance.Status.Reason = reason
			if retErr != nil {
				instance.Status.State = opsterv1.OpensearchTenantError
			}
			// Requeue after is 10 seconds if waiting for OpenSearch cluster
			if retResult.Requeue && retResult.RequeueAfter == 10*time.Second {
				instance.Status.State = opsterv1.OpensearchTenantPending
			}
			// Requeue is after 30 seconds for normal reconciliation after creation/update
			if retErr == nil && retResult.RequeueAfter == 30*time.Second {
				instance.Status.State = opsterv1.OpensearchTenantCreated
			}
			if reason == opensearchTenantExists {
				instance.Status.State = opsterv1.OpensearchTenantIgnored
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
			reason = "cannot change the cluster an tenant refers to"
			retErr = fmt.Errorf("%s", reason)
			r.recorder.Event(r.instance, "Warning", opensearchRefMismatch, reason)
			return
		}
	} else {
		if pointer.BoolDeref(r.updateStatus, true) {
			retErr = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
				instance := object.(*opsterv1.OpensearchTenant)
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

	// Check tenant state to make sure we don't touch preexisting tenants
	if r.instance.Status.ExistingTenant == nil {
		var exists bool
		exists, retErr = services.TenantExists(r.ctx, r.osClient, r.instance.Name)
		if retErr != nil {
			reason = "failed to get tenant status from Opensearch API"
			r.logger.Error(retErr, reason)
			r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
			return
		}
		if pointer.BoolDeref(r.updateStatus, true) {
			retErr = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
				instance := object.(*opsterv1.OpensearchTenant)
				instance.Status.ExistingTenant = &exists
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

	// If tenant is existing do nothing
	if *r.instance.Status.ExistingTenant {
		reason = opensearchTenantExists
		return
	}

	tenant := requests.Tenant{
		Description: r.instance.Spec.Description,
	}

	shouldUpdate, retErr := services.ShouldUpdateTenant(r.ctx, r.osClient, r.instance.Name, tenant)
	if retErr != nil {
		reason = "failed to get tenant status from Opensearch API"
		r.logger.Error(retErr, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
		return
	}

	if !shouldUpdate {
		r.logger.V(1).Info(fmt.Sprintf("tenant %s is in sync", r.instance.Name))
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, retErr
	}

	retErr = services.CreateOrUpdateTenant(r.ctx, r.osClient, r.instance.Name, tenant)
	if retErr != nil {
		reason = "failed to update tenant with Opensearch API"
		r.logger.Error(retErr, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
	}

	r.recorder.Event(r.instance, "Normal", opensearchAPIUpdated, "tenant updated in opensearch")

	return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, retErr
}

func (r *TenantReconciler) Delete() error {
	// If we have never successfully reconciled we can just exit
	if r.instance.Status.ExistingTenant == nil {
		return nil
	}

	if *r.instance.Status.ExistingTenant {
		r.logger.Info("tenant was pre-existing; not deleting")
		return nil
	}

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

	exist, err := services.TenantExists(r.ctx, r.osClient, r.instance.Name)
	if err != nil {
		return err
	}
	if !exist {
		r.logger.V(1).Info("tenant already deleted from opensearch")
		return nil
	}

	return services.DeleteTenant(r.ctx, r.osClient, r.instance.Name)
}
