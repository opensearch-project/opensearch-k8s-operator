package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"time"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	"github.com/cisco-open/operator-tools/pkg/reconciler"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	opensearchIsmPolicyExists       = "ISM Policy already exists in Opensearch"
	opensearchIsmPolicyNameMismatch = "OpensearchISMPolicyNameMismatch"
	opensearchClusterRequeueAfter   = 10 * time.Second
	defaultRequeueAfter             = 30 * time.Second
)

type IsmPolicyReconciler struct {
	client k8s.K8sClient
	ReconcilerOptions
	ctx      context.Context
	osClient *services.OsClusterClient
	recorder record.EventRecorder
	instance *opsterv1.OpenSearchISMPolicy
	cluster  *opsterv1.OpenSearchCluster
	logger   logr.Logger
}

func NewIsmReconciler(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	instance *opsterv1.OpenSearchISMPolicy,
	opts ...ReconcilerOption,
) *IsmPolicyReconciler {
	options := ReconcilerOptions{}
	options.apply(opts...)
	return &IsmPolicyReconciler{
		client:            k8s.NewK8sClient(client, ctx, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "role"))),
		ReconcilerOptions: options,
		ctx:               ctx,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx).WithValues("reconciler", "ismpolicy"),
	}
}

func (r *IsmPolicyReconciler) Reconcile() (retResult ctrl.Result, retErr error) {
	var reason string
	var policyId string

	defer func() {
		if !pointer.BoolDeref(r.updateStatus, true) {
			return
		}
		// When the reconciler is done, figure out what the state of the resource
		// is and set it in the state field accordingly.
		err := r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			instance := object.(*opsterv1.OpenSearchISMPolicy)
			instance.Status.Reason = reason
			if retErr != nil {
				instance.Status.State = opsterv1.OpensearchISMPolicyError
			}
			// Requeue after is 10 seconds if waiting for OpenSearch cluster
			if retResult.Requeue && retResult.RequeueAfter == opensearchClusterRequeueAfter {
				instance.Status.State = opsterv1.OpensearchISMPolicyPending
			}
			if retErr == nil && retResult.Requeue {
				instance.Status.State = opsterv1.OpensearchISMPolicyCreated
				instance.Status.PolicyId = policyId
			}
			if reason == opensearchIsmPolicyExists {
				instance.Status.State = opsterv1.OpensearchISMPolicyIgnored
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
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: opensearchClusterRequeueAfter,
		}, retErr
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
		retErr = fmt.Errorf("%s", reason)
		r.recorder.Event(r.instance, "Warning", opensearchRefMismatch, reason)
		return ctrl.Result{
			Requeue: false,
		}, retErr
	}

	if pointer.BoolDeref(r.updateStatus, true) {
		retErr = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			object.(*opsterv1.OpenSearchISMPolicy).Status.ManagedCluster = &r.cluster.UID
		})
		if retErr != nil {
			reason = fmt.Sprintf("failed to update status: %s", retErr)
			r.recorder.Event(r.instance, "Warning", statusError, reason)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: opensearchClusterRequeueAfter,
			}, retErr
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

	r.osClient, retErr = util.CreateClientForCluster(r.client, r.ctx, r.cluster, r.osClientTransport)
	if retErr != nil {
		reason = "error creating opensearch client"
		r.recorder.Event(r.instance, "Warning", opensearchError, reason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: opensearchClusterRequeueAfter,
		}, retErr
	}

	// If PolicyID is not provided explicitly, use metadata.name by default
	policyId = r.instance.Name
	if r.instance.Spec.PolicyID != "" {
		policyId = r.instance.Spec.PolicyID
	}

	newPolicy, retErr := r.CreateISMPolicy()
	if retErr != nil {
		r.logger.Error(retErr, reason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, retErr
	}

	existingPolicy, retErr := services.GetPolicy(r.ctx, r.osClient, policyId)
	// If not exists, create
	if errors.Is(retErr, services.ErrNotFound) {
		request := requests.ISMPolicy{
			Policy: *newPolicy,
		}
		retErr = services.CreateISMPolicy(r.ctx, r.osClient, request, policyId)
		if retErr != nil {
			reason = "failed to create ism policy"
			r.logger.Error(retErr, reason)
			r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: defaultRequeueAfter,
			}, retErr
		}
		// Mark the ISM Policy as not pre-existing (created by the operator)
		retErr = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			object.(*opsterv1.OpenSearchISMPolicy).Status.ExistingISMPolicy = pointer.Bool(false)
		})
		if retErr != nil {
			reason = "failed to update custom resource object"
			r.logger.Error(retErr, reason)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: defaultRequeueAfter,
			}, retErr
		}

		r.recorder.Event(r.instance, "Normal", opensearchAPIUpdated, "policy successfully created in OpenSearch Cluster")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, nil
	}

	// If other error, report
	if retErr != nil {
		reason = "failed to get the ism policy from Opensearch API"
		r.logger.Error(retErr, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, retErr
	}

	// If the ISM policy exists in OpenSearch cluster and was not created by the operator, update the status and return
	if r.instance.Status.ExistingISMPolicy == nil || *r.instance.Status.ExistingISMPolicy {
		retErr = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			object.(*opsterv1.OpenSearchISMPolicy).Status.ExistingISMPolicy = pointer.Bool(true)
		})
		if retErr != nil {
			reason = "failed to update custom resource object"
			r.logger.Error(retErr, reason)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: defaultRequeueAfter,
			}, retErr
		}
		reason = "the ISM policy already exists in the OpenSearch cluster"
		r.logger.Error(errors.New(opensearchIsmPolicyExists), reason)
		r.recorder.Event(r.instance, "Warning", opensearchIsmPolicyExists, reason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, nil
	}

	// Return if there are no changes
	if r.instance.Spec.PolicyID == existingPolicy.PolicyID && cmp.Equal(*newPolicy, existingPolicy.Policy, cmpopts.EquateEmpty()) {
		r.logger.V(1).Info(fmt.Sprintf("user %s is in sync", r.instance.Name))
		r.recorder.Event(r.instance, "Normal", opensearchAPIUnchanged, "policy is in sync")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, nil
	}

	request := requests.ISMPolicy{
		Policy: *newPolicy,
	}
	retErr = services.UpdateISMPolicy(r.ctx, r.osClient, request, &existingPolicy.SequenceNumber, &existingPolicy.PrimaryTerm, existingPolicy.PolicyID)
	if retErr != nil {
		reason = "failed to update ism policy with Opensearch API"
		r.logger.Error(retErr, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, retErr
	}

	r.recorder.Event(r.instance, "Normal", opensearchAPIUpdated, "policy updated in opensearch")
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: defaultRequeueAfter,
	}, nil
}

func (r *IsmPolicyReconciler) CreateISMPolicy() (*requests.ISMPolicySpec, error) {
	policy := requests.ISMPolicySpec{
		DefaultState: r.instance.Spec.DefaultState,
		Description:  r.instance.Spec.Description,
	}
	if r.instance.Spec.ErrorNotification != nil {
		dest := requests.Destination{}
		if r.instance.Spec.ErrorNotification.Destination != nil {
			if r.instance.Spec.ErrorNotification.Destination.Amazon != nil {
				dest.Amazon = &requests.DestinationURL{
					URL: r.instance.Spec.ErrorNotification.Destination.Amazon.URL,
				}
			}
			if r.instance.Spec.ErrorNotification.Destination.Chime != nil {
				dest.Chime = &requests.DestinationURL{
					URL: r.instance.Spec.ErrorNotification.Destination.Chime.URL,
				}
			}
			if r.instance.Spec.ErrorNotification.Destination.Slack != nil {
				dest.Slack = &requests.DestinationURL{
					URL: r.instance.Spec.ErrorNotification.Destination.Slack.URL,
				}
			}
			if r.instance.Spec.ErrorNotification.Destination.CustomWebhook != nil {
				dest.CustomWebhook = &requests.DestinationURL{
					URL: r.instance.Spec.ErrorNotification.Destination.CustomWebhook.URL,
				}
			}
		}
		policy.ErrorNotification = &requests.ErrorNotification{
			Channel:         r.instance.Spec.ErrorNotification.Channel,
			Destination:     &dest,
			MessageTemplate: &requests.MessageTemplate{Source: r.instance.Spec.ErrorNotification.MessageTemplate.Source},
		}
	}

	if r.instance.Spec.ISMTemplate != nil {
		ismTemplate := &requests.ISMTemplate{}
		ismTemplate.IndexPatterns = r.instance.Spec.ISMTemplate.IndexPatterns
		ismTemplate.Priority = r.instance.Spec.ISMTemplate.Priority
		policy.ISMTemplate = append(policy.ISMTemplate, *ismTemplate)
	}

	if len(r.instance.Spec.States) > 0 {
		policy.States = make([]requests.State, 0, len(r.instance.Spec.States))
		for _, state := range r.instance.Spec.States {
			actions := make([]requests.Action, 0, len(state.Actions))
			for _, action := range state.Actions {
				var replicaCount *requests.ReplicaCount
				if action.ReplicaCount != nil {
					replicaCount = &requests.ReplicaCount{
						NumberOfReplicas: action.ReplicaCount.NumberOfReplicas,
					}
				}
				var closea *requests.Close
				if action.Close != nil {
					closea = &requests.Close{}
				}
				var alias *requests.Alias

				if action.Alias != nil {
					alias = &requests.Alias{}
					aliasActions := make([]requests.AliasAction, 0, len(action.Alias.Actions))

					for _, aliasAction := range action.Alias.Actions {
						newAction := requests.AliasAction{}
						newAliasDetails := requests.AliasDetails{}

						copyAliasDetails := func(src *opsterv1.AliasDetails) {
							newAliasDetails.Aliases = src.Aliases
							newAliasDetails.Index = src.Index
							newAliasDetails.IsWriteIndex = src.IsWriteIndex
							newAliasDetails.Routing = src.Routing
						}

						if aliasAction.Add != nil {
							copyAliasDetails(aliasAction.Add)
							newAction.Add = &newAliasDetails
						}

						if aliasAction.Remove != nil {
							copyAliasDetails(aliasAction.Remove)
							newAction.Remove = &newAliasDetails
						}

						aliasActions = append(aliasActions, newAction)
					}

					alias.Actions = aliasActions
				}
				var rollover *requests.Rollover
				if action.Rollover != nil {
					rollover = &requests.Rollover{}
					if action.Rollover.MinDocCount != nil {
						rollover.MinDocCount = action.Rollover.MinDocCount
					}
					if action.Rollover.MinIndexAge != nil {
						rollover.MinIndexAge = action.Rollover.MinIndexAge
					}
					if action.Rollover.MinSize != nil {
						rollover.MinSize = action.Rollover.MinSize
					}
					if action.Rollover.MinPrimaryShardSize != nil {
						rollover.MinPrimaryShardSize = action.Rollover.MinPrimaryShardSize
					}
				}

				var del *requests.Delete
				if action.Delete != nil {
					del = &requests.Delete{}
				}
				var open *requests.Open
				if action.Open != nil {
					open = &requests.Open{}
				}
				var shrink *requests.Shrink
				if action.Shrink != nil {
					shrink = &requests.Shrink{}
					if action.Shrink.ForceUnsafe != nil {
						shrink.ForceUnsafe = action.Shrink.ForceUnsafe
					}
					if action.Shrink.MaxShardSize == nil && action.Shrink.NumNewShards == nil && action.Shrink.PercentageOfSourceShards == nil {
						reason := "either of MaxShardSize or NumNewShards or PercentageOfSourceShards is required"
						r.recorder.Event(r.instance, "Error", opensearchCustomResourceError, reason)
						return nil, errors.New(reason)
					}

					if action.Shrink.MaxShardSize != nil {
						if action.Shrink.NumNewShards == nil && action.Shrink.PercentageOfSourceShards == nil {
							shrink.MaxShardSize = action.Shrink.MaxShardSize
						} else {
							reason := "maxShardSize can't exist with NumNewShards or PercentageOfSourceShards. Keep one of these"
							r.recorder.Event(r.instance, "Error", opensearchCustomResourceError, reason)
							return nil, errors.New(reason)
						}
						if action.Shrink.NumNewShards != nil {
							if action.Shrink.MaxShardSize == nil && action.Shrink.PercentageOfSourceShards == nil {
								shrink.NumNewShards = action.Shrink.NumNewShards
							} else {
								reason := "numNewShards can't exist with MaxShardSize or PercentageOfSourceShards. Keep one of these"
								r.recorder.Event(r.instance, "Error", opensearchCustomResourceError, reason)
								return nil, errors.New(reason)
							}
						}
						if action.Shrink.PercentageOfSourceShards != nil {
							if action.Shrink.NumNewShards == nil && action.Shrink.MaxShardSize == nil {
								shrink.PercentageOfSourceShards = action.Shrink.PercentageOfSourceShards
							} else {
								reason := "percentageOfSourceShards can't exist with MaxShardSize or NumNewShards. Keep one of these"
								r.recorder.Event(r.instance, "Error", opensearchCustomResourceError, reason)
								return nil, errors.New(reason)
							}
						}
						if action.Shrink.TargetIndexNameTemplate != nil {
							shrink.TargetIndexNameTemplate = action.Shrink.TargetIndexNameTemplate
						}
					}
				}

				var forceMerge *requests.ForceMerge
				if action.ForceMerge != nil {
					forceMerge = &requests.ForceMerge{MaxNumSegments: action.ForceMerge.MaxNumSegments}
				}
				var alloc *requests.Allocation
				if action.Allocation != nil {
					alloc = &requests.Allocation{
						Exclude: action.Allocation.Exclude,
						Include: action.Allocation.Include,
						Require: action.Allocation.Require,
						WaitFor: action.Allocation.WaitFor,
					}
				}
				var indexPri *requests.IndexPriority
				if action.IndexPriority != nil {
					indexPri = &requests.IndexPriority{
						Priority: action.IndexPriority.Priority,
					}
				}
				var snapshot *requests.Snapshot
				if action.Snapshot != nil {
					snapshot = &requests.Snapshot{
						Repository: action.Snapshot.Repository,
						Snapshot:   action.Snapshot.Snapshot,
					}
				}

				var retry *requests.Retry
				if action.Retry != nil {
					retry = &requests.Retry{
						Backoff: action.Retry.Backoff,
						Delay:   action.Retry.Delay,
						Count:   action.Retry.Count,
					}
				} else {
					retry = &requests.Retry{
						Backoff: "exponential",
						Count:   3,
						Delay:   "1m",
					}
				}
				var timeOut *string
				if action.Timeout != nil {
					timeOut = action.Timeout
				}
				var readWrite *requests.ReadWrite
				if action.ReadWrite != nil {
					readWrite = &requests.ReadWrite{}
				}
				var readOnly *requests.ReadOnly
				if action.ReadOnly != nil {
					readOnly = &requests.ReadOnly{}
				}
				actions = append(actions, requests.Action{
					ReplicaCount:  replicaCount,
					Retry:         retry,
					Close:         closea,
					Delete:        del,
					Open:          open,
					Shrink:        shrink,
					Snapshot:      snapshot,
					Allocation:    alloc,
					ForceMerge:    forceMerge,
					Rollover:      rollover,
					IndexPriority: indexPri,
					Timeout:       timeOut,
					ReadOnly:      readOnly,
					ReadWrite:     readWrite,
					Alias:         alias,
				})
			}
			transitions := make([]requests.Transition, 0, len(state.Transitions))
			for _, transition := range state.Transitions {
				conditions := requests.Condition{}
				if transition.Conditions.MinDocCount != nil {
					conditions.MinDocCount = transition.Conditions.MinDocCount
				}
				if transition.Conditions.MinIndexAge != nil {
					conditions.MinIndexAge = transition.Conditions.MinIndexAge
				}
				if transition.Conditions.MinSize != nil {
					conditions.MinSize = transition.Conditions.MinSize
				}
				if transition.Conditions.MinRolloverAge != nil {
					conditions.MinRolloverAge = transition.Conditions.MinRolloverAge
				}
				if transition.Conditions.Cron != nil {
					conditions.Cron = &requests.Cron{
						CronDetails: &requests.CronDetails{
							Expression: transition.Conditions.Cron.CronDetails.Expression,
							Timezone:   transition.Conditions.Cron.CronDetails.Timezone,
						},
					}
				}
				statename := transition.StateName
				transitions = append(transitions, requests.Transition{Conditions: conditions, StateName: statename})
			}
			policy.States = append(policy.States, requests.State{Actions: actions, Name: state.Name, Transitions: transitions})
		}
	}

	return &policy, nil
}

// Delete ISM policy from the OS cluster
func (r *IsmPolicyReconciler) Delete() error {
	// If we have never successfully reconciled we can just exit
	if r.instance.Status.ExistingISMPolicy == nil {
		return nil
	}

	if *r.instance.Status.ExistingISMPolicy {
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

	// If PolicyID not provided explicitly, use metadata.name by default
	policyId := r.instance.Spec.PolicyID
	if policyId == "" {
		policyId = r.instance.Name
	}

	err = services.DeleteISMPolicy(r.ctx, r.osClient, policyId)
	if err != nil {
		return err
	}
	return nil
}
