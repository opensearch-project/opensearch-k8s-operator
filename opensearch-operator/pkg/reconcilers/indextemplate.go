package reconcilers

import (
	"context"
	"fmt"
	"time"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
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

const (
	opensearchIndexTemplateExists       = "index template already exists in OpenSearch; not modifying"
	opensearchIndexTemplateNameMismatch = "OpensearchIndexTemplateNameMismatch"
)

type IndexTemplateReconciler struct {
	client k8s.K8sClient
	ReconcilerOptions
	ctx      context.Context
	osClient *services.OsClusterClient
	recorder record.EventRecorder
	instance *opsterv1.OpensearchIndexTemplate
	cluster  *opsterv1.OpenSearchCluster
	logger   logr.Logger
}

func NewIndexTemplateReconciler(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	instance *opsterv1.OpensearchIndexTemplate,
	opts ...ReconcilerOption,
) *IndexTemplateReconciler {
	options := ReconcilerOptions{}
	options.apply(opts...)
	return &IndexTemplateReconciler{
		client:            k8s.NewK8sClient(client, ctx, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "role"))),
		ReconcilerOptions: options,
		ctx:               ctx,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx).WithValues("reconciler", "indextemplate"),
	}
}

func (r *IndexTemplateReconciler) Reconcile() (result ctrl.Result, err error) {
	var reason string
	var templateName string

	defer func() {
		if !pointer.BoolDeref(r.updateStatus, true) {
			return
		}
		// When the reconciler is done, figure out what the state of the resource
		// is and set it in the state field accordingly.
		err := r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			instance := object.(*opsterv1.OpensearchIndexTemplate)
			instance.Status.Reason = reason
			if err != nil {
				instance.Status.State = opsterv1.OpensearchIndexTemplateError
			}
			if result.Requeue && result.RequeueAfter == 10*time.Second {
				instance.Status.State = opsterv1.OpensearchIndexTemplatePending
			}
			if err == nil && result.RequeueAfter == 30*time.Second {
				instance.Status.State = opsterv1.OpensearchIndexTemplateCreated
				instance.Status.IndexTemplateName = templateName
			}
			if reason == opensearchIndexTemplateExists {
				instance.Status.State = opsterv1.OpensearchIndexTemplateIgnored
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
	if r.instance.Status.ManagedCluster != nil {
		if *r.instance.Status.ManagedCluster != r.cluster.UID {
			reason = "cannot change the cluster an index template refers to"
			err = fmt.Errorf("%s", reason)
			r.recorder.Event(r.instance, "Warning", opensearchRefMismatch, reason)
			return
		}
	} else {
		if pointer.BoolDeref(r.updateStatus, true) {
			err = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
				instance := object.(*opsterv1.OpensearchIndexTemplate)
				instance.Status.ManagedCluster = &r.cluster.UID
			})
			if err != nil {
				reason = fmt.Sprintf("failed to update status: %s", err)
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
	if r.instance.Spec.Name != "" {
		templateName = r.instance.Spec.Name
	}

	// Check index template state to make sure we don't touch preexisting index templates
	if r.instance.Status.ExistingIndexTemplate == nil {
		var exists bool
		exists, err = services.IndexTemplateExists(r.ctx, r.osClient, templateName)
		if err != nil {
			reason = "failed to get index template status from OpenSearch API"
			r.logger.Error(err, reason)
			r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
			return
		}
		if pointer.BoolDeref(r.updateStatus, true) {
			err = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
				instance := object.(*opsterv1.OpensearchIndexTemplate)
				instance.Status.ExistingIndexTemplate = &exists
			})
			if err != nil {
				reason = fmt.Sprintf("failed to update status: %s", err)
				r.recorder.Event(r.instance, "Warning", statusError, reason)
				return
			}
		} else {
			// Emit an event for unit testing assertion
			r.recorder.Event(r.instance, "Normal", "UnitTest", fmt.Sprintf("exists is %t", exists))
			return
		}
	}

	// If index template is existing do nothing
	if *r.instance.Status.ExistingIndexTemplate {
		reason = opensearchIndexTemplateExists
		return
	}

	// the template name is immutable, so check the old name (r.instance.Status.IndexTemplateName) against the new
	if r.instance.Status.IndexTemplateName != "" && templateName != r.instance.Status.IndexTemplateName {
		reason = "cannot change the index template name"
		err = fmt.Errorf("%s", reason)
		r.recorder.Event(r.instance, "Warning", opensearchIndexTemplateNameMismatch, reason)
		return
	}

	// rewrite the CRD format to the gateway format
	resource := helpers.TranslateIndexTemplateToRequest(r.instance.Spec)

	shouldUpdate, err := services.ShouldUpdateIndexTemplate(r.ctx, r.osClient, templateName, resource)
	if err != nil {
		reason = "failed to get index template status from OpenSearch API"
		r.logger.Error(err, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
		return
	}

	if !shouldUpdate {
		r.logger.V(1).Info(fmt.Sprintf("index template %s is in sync", r.instance.Name))
		result = ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}
		return
	}

	err = services.CreateOrUpdateIndexTemplate(r.ctx, r.osClient, templateName, resource)
	if err != nil {
		reason = "failed to update index template with OpenSearch API"
		r.logger.Error(err, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
	}

	r.recorder.Event(r.instance, "Normal", opensearchAPIUpdated, "index template updated in opensearch")

	result = ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}
	return
}

func (r *IndexTemplateReconciler) Delete() error {
	// If we have never successfully reconciled we can just exit
	if r.instance.Status.ExistingIndexTemplate == nil {
		return nil
	}

	if *r.instance.Status.ExistingIndexTemplate {
		r.logger.Info("index template was pre-existing; not deleting")
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

	templateName := r.instance.Name
	if r.instance.Spec.Name != "" {
		templateName = r.instance.Spec.Name
	}

	exist, err := services.IndexTemplateExists(r.ctx, r.osClient, templateName)
	if err != nil {
		return err
	}
	if !exist {
		r.logger.V(1).Info("index template already deleted from opensearch")
		return nil
	}

	return services.DeleteIndexTemplate(r.ctx, r.osClient, templateName)
}
