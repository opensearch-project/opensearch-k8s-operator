package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"k8s.io/utils/ptr"
	"time"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconciler"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	opensearchSnapshotPolicyExists = "snapshot policy already exists in OpenSearch; not modifying"
)

type SnapshotPolicyReconciler struct {
	client k8s.K8sClient
	ReconcilerOptions
	ctx      context.Context
	osClient *services.OsClusterClient
	recorder record.EventRecorder
	instance *opsterv1.OpensearchSnapshotPolicy
	cluster  *opsterv1.OpenSearchCluster
	logger   logr.Logger
}

func NewSnapshotPolicyReconciler(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	instance *opsterv1.OpensearchSnapshotPolicy,
	opts ...ReconcilerOption,
) *SnapshotPolicyReconciler {
	options := ReconcilerOptions{}
	options.apply(opts...)
	return &SnapshotPolicyReconciler{
		client:            k8s.NewK8sClient(client, ctx, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "role"))),
		ReconcilerOptions: options,
		ctx:               ctx,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx).WithValues("reconciler", "snapshotpolicy"),
	}
}

func (r *SnapshotPolicyReconciler) Reconcile() (result ctrl.Result, err error) {
	var reason string
	var policyName string

	defer func() {
		if !ptr.Deref(r.updateStatus, true) {
			return
		}
		// When the reconciler is done, figure out what the state of the resource
		// is and set it in the state field accordingly.
		err := r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			instance := object.(*opsterv1.OpensearchSnapshotPolicy)
			instance.Status.Reason = reason
			if err != nil {
				instance.Status.State = opsterv1.OpensearchSnapshotPolicyError
			}
			if result.Requeue && result.RequeueAfter == 10*time.Second {
				instance.Status.State = opsterv1.OpensearchSnapshotPolicyPending
			}
			if err == nil && result.RequeueAfter == 30*time.Second {
				instance.Status.State = opsterv1.OpensearchSnapshotPolicyCreated
				instance.Status.SnapshotPolicyName = policyName
			}
			if reason == opensearchSnapshotPolicyExists {
				instance.Status.State = opsterv1.OpensearchSnapshotPolicyIgnored
			}
		})

		if err != nil {
			r.logger.Error(err, "failed to update status")
		}
	}()

	r.cluster, err = util.FetchOpensearchCluster(r.client, r.ctx, types.NamespacedName{
		Name:      r.instance.Spec.OpensearchRef.Name,
		Namespace: r.instance.Namespace,
	})
	if err != nil {
		reason = "error fetching opensearch cluster"
		r.logger.Error(err, "failed to fetch opensearch cluster")
		r.recorder.Event(r.instance, "Warning", opensearchError, reason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: opensearchClusterRequeueAfter,
		}, err
	}
	if r.cluster == nil {
		r.logger.Info("opensearch cluster does not exist, requeueing")
		reason = "waiting for opensearch cluster to exist"
		r.recorder.Event(r.instance, "Normal", opensearchPending, reason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: opensearchClusterRequeueAfter,
		}, nil
	}

	// Check cluster ref has not changed
	managedCluster := r.instance.Status.ManagedCluster
	if managedCluster != nil && *managedCluster != r.cluster.UID {
		reason = "cannot change the cluster a resource refers to"
		err = fmt.Errorf("%s", reason)
		r.recorder.Event(r.instance, "Warning", opensearchRefMismatch, reason)
		return ctrl.Result{
			Requeue: false,
		}, err
	}

	if ptr.Deref(r.updateStatus, true) {
		err = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			object.(*opsterv1.OpensearchSnapshotPolicy).Status.ManagedCluster = &r.cluster.UID
		})
		if err != nil {
			reason = fmt.Sprintf("failed to update status: %s", err)
			r.recorder.Event(r.instance, "Warning", statusError, reason)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: opensearchClusterRequeueAfter,
			}, err
		}
	}

	// Check cluster is ready
	if r.cluster.Status.Phase != opsterv1.PhaseRunning {
		r.logger.Info("opensearch cluster is not running, requeueing")
		reason = "waiting for opensearch cluster status to be running"
		r.recorder.Event(r.instance, "Normal", opensearchPending, reason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: opensearchClusterRequeueAfter,
		}, nil
	}

	r.osClient, err = util.CreateClientForCluster(r.client, r.ctx, r.cluster, r.osClientTransport)
	if err != nil {
		reason = "error creating opensearch client"
		r.recorder.Event(r.instance, "Warning", opensearchError, reason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: opensearchClusterRequeueAfter,
		}, err
	}

	// If policyName is not provided explicitly, use metadata.name by default
	policyName = r.instance.Name
	if r.instance.Spec.PolicyName != "" {
		policyName = r.instance.Spec.PolicyName
	}

	newPolicy, err := r.CreateSnapshotPolicy()
	if err != nil {
		shortReason := "failed to generate snapshot policy document"
		reason = fmt.Sprintf("%s: %s", shortReason, err.Error())
		r.logger.Error(err, shortReason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 30 * time.Second,
		}, err
	}

	existingPolicy, err := services.GetSnapshotPolicy(r.ctx, r.osClient, policyName)
	// If not exists, create
	if errors.Is(err, services.ErrNotFound) {
		request := requests.SnapshotPolicy{
			Policy: *newPolicy,
		}
		err = services.CreateSnapshotPolicy(r.ctx, r.osClient, request, policyName)
		if err != nil {
			shortReason := "failed to create snapshot policy"
			reason = fmt.Sprintf("%s: %s", shortReason, err.Error())
			r.logger.Error(err, shortReason)
			r.recorder.Event(r.instance, "Warning", opensearchAPIError, shortReason)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: defaultRequeueAfter,
			}, err
		}
		// Mark the Snapshot Policy as not pre-existing (created by the operator)
		err = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			object.(*opsterv1.OpensearchSnapshotPolicy).Status.ExistingSnapshotPolicy = ptr.To(false)
		})
		if err != nil {
			reason = "failed to update custom resource object"
			r.logger.Error(err, reason)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: defaultRequeueAfter,
			}, err
		}

		r.recorder.Event(r.instance, "Normal", opensearchAPIUpdated, "policy successfully created in OpenSearch Cluster")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, nil
	}

	// If other error, report
	if err != nil {
		reason = "failed to get the snapshot policy from Opensearch API"
		r.logger.Error(err, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, err
	}

	// If the Snapshot policy exists in OpenSearch cluster and was not created by the operator, update the status and return
	if r.instance.Status.ExistingSnapshotPolicy == nil || *r.instance.Status.ExistingSnapshotPolicy {
		err = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			object.(*opsterv1.OpensearchSnapshotPolicy).Status.ExistingSnapshotPolicy = ptr.To(true)
		})
		if err != nil {
			reason = "failed to update custom resource object"
			r.logger.Error(err, reason)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: defaultRequeueAfter,
			}, err
		}
		reason = "the Snapshot policy already exists in the OpenSearch cluster"
		r.logger.Error(errors.New(opensearchSnapshotPolicyExists), reason)
		r.recorder.Event(r.instance, "Warning", opensearchSnapshotPolicyExists, reason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, nil
	}

	// Return if there are no changes
	if r.instance.Status.SnapshotPolicyName == existingPolicy.Policy.PolicyName && cmp.Equal(*newPolicy, existingPolicy.Policy, cmpopts.EquateEmpty()) {
		r.logger.V(1).Info(fmt.Sprintf("policy %s is in sync", r.instance.Spec.PolicyName))
		r.recorder.Event(r.instance, "Normal", opensearchAPIUnchanged, "policy is in sync")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, nil
	}
	request := requests.SnapshotPolicy{
		Policy: *newPolicy,
	}

	err = services.UpdateSnapshotPolicy(r.ctx, r.osClient, request, &existingPolicy.SequenceNumber, &existingPolicy.PrimaryTerm, existingPolicy.Policy.PolicyName)
	if err != nil {
		shortReason := "failed to update snapshotpolicy policy with Opensearch API"
		reason = fmt.Sprintf("%s: %s", shortReason, err.Error())
		r.logger.Error(err, shortReason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, shortReason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, err
	}

	r.recorder.Event(r.instance, "Normal", opensearchAPIUpdated, "policy updated in opensearch")
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: defaultRequeueAfter,
	}, nil
}

func (r *SnapshotPolicyReconciler) CreateSnapshotPolicy() (*requests.SnapshotPolicySpec, error) {
	policy := requests.SnapshotPolicySpec{
		PolicyName:  r.instance.Spec.PolicyName,
		Description: r.instance.Spec.Description,
		Enabled:     r.instance.Spec.Enabled,
		SnapshotConfig: requests.SnapshotConfig{
			DateFormat:         r.instance.Spec.SnapshotConfig.DateFormat,
			DateFormatTimezone: r.instance.Spec.SnapshotConfig.DateFormatTimezone,
			Indices:            r.instance.Spec.SnapshotConfig.Indices,
			Repository:         r.instance.Spec.SnapshotConfig.Repository,
			IgnoreUnavailable:  r.instance.Spec.SnapshotConfig.IgnoreUnavailable,
			IncludeGlobalState: r.instance.Spec.SnapshotConfig.IncludeGlobalState,
			Partial:            r.instance.Spec.SnapshotConfig.Partial,
			Metadata:           r.instance.Spec.SnapshotConfig.Metadata,
		},
		Creation: requests.SnapshotCreation{
			Schedule: requests.CronSchedule{
				Cron: requests.CronExpression{
					Expression: r.instance.Spec.Creation.Schedule.Cron.Expression,
					Timezone:   r.instance.Spec.Creation.Schedule.Cron.Timezone,
				},
			},
			TimeLimit: r.instance.Spec.Creation.TimeLimit,
		},
	}

	if r.instance.Spec.Deletion != nil {
		del := &requests.SnapshotDeletion{
			TimeLimit: r.instance.Spec.Deletion.TimeLimit,
		}

		if r.instance.Spec.Deletion.Schedule != nil {
			del.Schedule = &requests.CronSchedule{
				Cron: requests.CronExpression{
					Expression: r.instance.Spec.Deletion.Schedule.Cron.Expression,
					Timezone:   r.instance.Spec.Deletion.Schedule.Cron.Timezone,
				},
			}
		}

		if r.instance.Spec.Deletion.DeleteCondition != nil {
			del.DeleteCondition = &requests.SnapshotDeleteCondition{
				MaxCount: r.instance.Spec.Deletion.DeleteCondition.MaxCount,
				MaxAge:   r.instance.Spec.Deletion.DeleteCondition.MaxAge,
				MinCount: r.instance.Spec.Deletion.DeleteCondition.MinCount,
			}
		}

		policy.Deletion = del
	}

	if r.instance.Spec.Notification != nil {
		notification := &requests.SnapshotNotification{
			Channel: requests.NotificationChannel{
				ID: r.instance.Spec.Notification.Channel.ID,
			},
		}

		if r.instance.Spec.Notification.Conditions != nil {
			notification.Conditions = &requests.NotificationConditions{
				Creation: r.instance.Spec.Notification.Conditions.Creation,
				Deletion: r.instance.Spec.Notification.Conditions.Deletion,
				Failure:  r.instance.Spec.Notification.Conditions.Failure,
			}
		}

		policy.Notification = notification
	}

	return &policy, nil
}

func (r *SnapshotPolicyReconciler) Delete() error {
	// If we have never successfully reconciled we can just exit
	if r.instance.Status.ExistingSnapshotPolicy == nil {
		return nil
	}

	if *r.instance.Status.ExistingSnapshotPolicy {
		r.logger.Info("policy was pre-existing; not deleting")
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

	// If PolicyName not provided explicitly, use metadata.name by default
	policyName := r.instance.Spec.PolicyName
	if policyName == "" {
		policyName = r.instance.Name
	}

	err = services.DeleteSnapshotPolicy(r.ctx, r.osClient, policyName)
	if err != nil {
		r.logger.Error(err, "failed to delete snapshot policy")
		return err
	} else {
		r.logger.Info("snapshot policy deleted")
	}
	return nil
}
