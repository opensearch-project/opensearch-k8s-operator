package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"k8s.io/utils/ptr"

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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	opensearchSearchTemplateExists       = "search template already exists in OpenSearch; not modifying"
	opensearchSearchTemplateNameMismatch = "OpensearchSearchTemplateNameMismatch"
)

type SearchTemplateReconciler struct {
	client k8s.K8sClient
	ReconcilerOptions
	ctx      context.Context
	osClient *services.OsClusterClient
	recorder record.EventRecorder
	instance *opsterv1.OpensearchSearchTemplate
	cluster  *opsterv1.OpenSearchCluster
	logger   logr.Logger
}

func NewSearchTemplateReconciler(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	instance *opsterv1.OpensearchSearchTemplate,
	opts ...ReconcilerOption,
) *SearchTemplateReconciler {
	options := ReconcilerOptions{}
	options.apply(opts...)
	return &SearchTemplateReconciler{
		client:            k8s.NewK8sClient(client, ctx, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "role"))),
		ReconcilerOptions: options,
		ctx:               ctx,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx).WithValues("reconciler", "searchtemplate"),
	}
}

func (r *SearchTemplateReconciler) Reconcile() (result ctrl.Result, err error) {
	var reason string
	var templateName string

	defer func() {
		if !ptr.Deref(r.updateStatus, true) {
			return
		}
		// When the reconciler is done, figure out what the state of the resource
		// is and set it in the state field accordingly.
		err := r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			instance := object.(*opsterv1.OpensearchSearchTemplate)
			instance.Status.Reason = reason
			if err != nil {
				instance.Status.State = opsterv1.OpensearchSearchTemplateError
			}
			if result.Requeue && result.RequeueAfter == 10*time.Second {
				instance.Status.State = opsterv1.OpensearchSearchTemplatePending
			}
			if err == nil && result.RequeueAfter == 30*time.Second {
				instance.Status.State = opsterv1.OpensearchSearchTemplateCreated
				instance.Status.SearchTemplateName = templateName
			}
			if reason == opensearchSearchTemplateExists {
				instance.Status.State = opsterv1.OpensearchSearchTemplateIgnored
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
		return
	}

	if r.cluster == nil {
		r.logger.Info("opensearch cluster does not exist, requeueing")
		reason = "waiting for opensearch cluster to exist"
		r.recorder.Event(r.instance, "Normal", opensearchPending, reason)
		result = ctrl.Result{
			Requeue:      true,
			RequeueAfter: 10 * time.Second,
		}
		return
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
			object.(*opsterv1.OpensearchSearchTemplate).Status.ManagedCluster = &r.cluster.UID
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
		result = ctrl.Result{
			Requeue:      true,
			RequeueAfter: 10 * time.Second,
		}
		return
	}

	r.osClient, err = util.CreateClientForCluster(r.client, r.ctx, r.cluster, r.osClientTransport)
	if err != nil {
		reason = "error creating opensearch client"
		r.recorder.Event(r.instance, "Warning", opensearchError, reason)
		return
	}

	templateName = r.instance.Name
	if r.instance.Spec.ScriptId != "" {
		templateName = r.instance.Spec.ScriptId
	}

	newSearchTemplate, err := r.TranslateSearchTemplate()
	if err != nil {
		shortReason := "failed to generate search template template document"
		reason = fmt.Sprintf("%s: %s", shortReason, err.Error())
		r.logger.Error(err, shortReason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 30 * time.Second,
		}, err
	}

	existingSearchTemplate, err := services.GetSearchTemplate(r.ctx, r.osClient, templateName)
	// If not exists, create
	if errors.Is(err, services.ErrNotFound) {
		request := newSearchTemplate
		err = services.CreateSearchTemplate(r.ctx, r.osClient, *request, templateName)
		if err != nil {
			shortReason := "failed to create search template template"
			reason = fmt.Sprintf("%s: %s", shortReason, err.Error())
			r.logger.Error(err, shortReason)
			r.recorder.Event(r.instance, "Warning", opensearchAPIError, shortReason)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: defaultRequeueAfter,
			}, err
		}
		// Mark the Search SearchTemplate as not pre-existing (created by the operator)
		err = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			object.(*opsterv1.OpensearchSearchTemplate).Status.ExistingSearchTemplate = ptr.To(false)
		})
		if err != nil {
			reason = "failed to update custom resource object"
			r.logger.Error(err, reason)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: defaultRequeueAfter,
			}, err
		}

		r.recorder.Event(r.instance, "Normal", opensearchAPIUpdated, "template successfully created in OpenSearch Cluster")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, nil
	}

	// If other error, report
	if err != nil {
		reason = "failed to get the search template template from Opensearch API"
		r.logger.Error(err, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, err
	}

	// If the Search template exists in OpenSearch cluster and was not created by the operator, update the status and return
	if r.instance.Status.ExistingSearchTemplate == nil || *r.instance.Status.ExistingSearchTemplate {
		err = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			object.(*opsterv1.OpensearchSearchTemplate).Status.ExistingSearchTemplate = ptr.To(true)
		})
		if err != nil {
			reason = "failed to update custom resource object"
			r.logger.Error(err, reason)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: defaultRequeueAfter,
			}, err
		}
		reason = "the earch template already exists in the OpenSearch cluster"
		r.logger.Error(errors.New(opensearchSearchTemplateExists), reason)
		r.recorder.Event(r.instance, "Warning", opensearchSearchTemplateExists, reason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, nil
	}

	// Return if there are no changes
	if r.instance.Status.SearchTemplateName == existingSearchTemplate.PolicyId && cmp.Equal(*newSearchTemplate, existingSearchTemplate.PolicyId, cmpopts.EquateEmpty()) {
		r.logger.V(1).Info(fmt.Sprintf("template %s is in sync", r.instance.Name))
		r.recorder.Event(r.instance, "Normal", opensearchAPIUnchanged, "template is in sync")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, nil
	}
	request := newSearchTemplate

	err = services.UpdateSearchTemplate(r.ctx, r.osClient, *request, existingSearchTemplate.PolicyId)
	if err != nil {
		shortReason := "failed to update search template with Opensearch API"
		reason = fmt.Sprintf("%s: %s", shortReason, err.Error())
		r.logger.Error(err, shortReason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, shortReason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, err
	}

	r.recorder.Event(r.instance, "Normal", opensearchAPIUpdated, "template updated in opensearch")
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: defaultRequeueAfter,
	}, nil
}

// CreateSearchTemplate creates a search template in OpenSearch based on the instance's spec.
func (r *SearchTemplateReconciler) TranslateSearchTemplate() (*requests.SearchTemplateSpec, error) {
	spec := requests.SearchTemplateSpec{
		ScriptId: r.instance.Spec.ScriptId,
		Params:   r.instance.Spec.Params,
		Script: requests.SearchTemplateScript{
			AllowNoIndices:             r.instance.Spec.Script.AllowNoIndices,
			AllowPartialSearchResults:  r.instance.Spec.Script.AllowPartialSearchResults,
			Analyzer:                   r.instance.Spec.Script.Analyzer,
			AnalyzeWildcard:            r.instance.Spec.Script.AnalyzeWildcard,
			BatchedReduceSize:          r.instance.Spec.Script.BatchedReduceSize,
			CancelAfterTimeInterval:    r.instance.Spec.Script.CancelAfterTimeInterval,
			CCSMinimizeRoundtrips:      r.instance.Spec.Script.CCSMinimizeRoundtrips,
			DefaultOperator:            r.instance.Spec.Script.DefaultOperator,
			DF:                         r.instance.Spec.Script.DF,
			DocvalueFields:             r.instance.Spec.Script.DocvalueFields,
			ExpandWildcards:            r.instance.Spec.Script.ExpandWildcards,
			Explain:                    r.instance.Spec.Script.Explain,
			From:                       r.instance.Spec.Script.From,
			IgnoreThrottled:            r.instance.Spec.Script.IgnoreThrottled,
			IgnoreUnavailable:          r.instance.Spec.Script.IgnoreUnavailable,
			Lenient:                    r.instance.Spec.Script.Lenient,
			MaxConcurrentShardRequests: r.instance.Spec.Script.MaxConcurrentShardRequests,
			PhaseTook:                  r.instance.Spec.Script.PhaseTook,
			PreFilterShardSize:         r.instance.Spec.Script.PreFilterShardSize,
			Preference:                 r.instance.Spec.Script.Preference,
			Q:                          r.instance.Spec.Script.Q,
			RequestCache:               r.instance.Spec.Script.RequestCache,
			RestTotalHitsAsInt:         r.instance.Spec.Script.RestTotalHitsAsInt,
			Routing:                    r.instance.Spec.Script.Routing,
			Scroll:                     r.instance.Spec.Script.Scroll,
			SearchType:                 r.instance.Spec.Script.SearchType,
			SeqNoPrimaryTerm:           r.instance.Spec.Script.SeqNoPrimaryTerm,
			Size:                       r.instance.Spec.Script.Size,
			Sort:                       r.instance.Spec.Script.Sort,
			Source:                     r.instance.Spec.Script.Source,
			SourceExcludes:             r.instance.Spec.Script.SourceExcludes,
			SourceIncludes:             r.instance.Spec.Script.SourceIncludes,
			Stats:                      r.instance.Spec.Script.Stats,
			StoredFields:               r.instance.Spec.Script.StoredFields,
			SuggestField:               r.instance.Spec.Script.SuggestField,
			SuggestMode:                r.instance.Spec.Script.SuggestMode,
			SuggestSize:                r.instance.Spec.Script.SuggestSize,
			SuggestText:                r.instance.Spec.Script.SuggestText,
			TerminateAfter:             r.instance.Spec.Script.TerminateAfter,
			Timeout:                    r.instance.Spec.Script.Timeout,
			TrackScores:                r.instance.Spec.Script.TrackScores,
			TrackTotalHits:             r.instance.Spec.Script.TrackTotalHits,
			TypedKeys:                  r.instance.Spec.Script.TypedKeys,
			Version:                    r.instance.Spec.Script.Version,
			IncludeNamedQueriesScore:   r.instance.Spec.Script.IncludeNamedQueriesScore,
		},
	}
	return &spec, nil
}

// Delete removes the search template from OpenSearch if it was created by the operator.
func (r *SearchTemplateReconciler) Delete() error {
	// If the search template was not created by the operator, skip deletion
	if r.instance.Status.ExistingSearchTemplate == nil || *r.instance.Status.ExistingSearchTemplate {
		r.logger.Info("search template was pre-existing; not deleting")
		return nil
	}

	var err error

	// Fetch the associated OpenSearch cluster
	r.cluster, err = util.FetchOpensearchCluster(r.client, r.ctx, types.NamespacedName{
		Name:      r.instance.Spec.OpensearchRef.Name,
		Namespace: r.instance.Namespace,
	})
	if err != nil {
		r.logger.Error(err, "failed to fetch opensearch cluster")
		return err
	}

	// If the OpenSearch cluster doesn't exist or is being deleted, skip deletion
	if r.cluster == nil || !r.cluster.DeletionTimestamp.IsZero() {
		r.logger.Info("opensearch cluster does not exist or is being deleted; skipping search template deletion")
		return nil
	}

	// Create OpenSearch client for the cluster
	r.osClient, err = util.CreateClientForCluster(r.client, r.ctx, r.cluster, r.osClientTransport)
	if err != nil {
		r.logger.Error(err, "failed to create opensearch client")
		return err
	}

	// Determine the name of the search template
	templateName := r.instance.Name
	if r.instance.Spec.ScriptId != "" {
		templateName = r.instance.Spec.ScriptId
	}

	// Check if the search template exists in OpenSearch
	searchTemplate, err := services.GetSearchTemplate(r.ctx, r.osClient, templateName)
	exist := searchTemplate.Found
	if err != nil {
		r.logger.Error(err, "failed to check if search template exists")
		return err
	}

	// If the search template does not exist, log and return
	if !exist {
		r.logger.Info("search template already deleted from opensearch")
		return nil
	}

	// Delete the search template from OpenSearch
	err = services.DeleteSearchTemplate(r.ctx, r.osClient, templateName)
	if err != nil {
		r.logger.Error(err, "failed to delete search template from opensearch")
		return err
	}

	r.logger.Info("search template successfully deleted from opensearch")
	return nil
}
