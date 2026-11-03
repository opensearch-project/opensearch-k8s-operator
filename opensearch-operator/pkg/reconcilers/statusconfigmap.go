package reconcilers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconciler"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// statusConfigMapLastUpdateAnnotation records the last write time on the
// ConfigMap itself, so the interval check survives controller restarts.
const statusConfigMapLastUpdateAnnotation = "opensearch.org/status-last-updated"

const statusConfigMapReconcilerName = "status_configmap"

type StatusConfigMapReconciler struct {
	client k8s.K8sClient
	ReconcilerOptions
	ctx      context.Context
	osClient *services.OsClusterClient
	recorder record.EventRecorder
	instance *opensearchv1.OpenSearchCluster
	logger   logr.Logger
}

func NewStatusConfigMapReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	instance *opensearchv1.OpenSearchCluster,
	opts ...ReconcilerOption,
) *StatusConfigMapReconciler {
	options := ReconcilerOptions{}
	options.apply(opts...)
	return &StatusConfigMapReconciler{
		client:            k8s.NewK8sClient(client, ctx, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", statusConfigMapReconcilerName))),
		ReconcilerOptions: options,
		ctx:               ctx,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx).WithValues("reconciler", statusConfigMapReconcilerName),
	}
}

// Reconcile creates or updates the status ConfigMap when enabled and the update
// interval has elapsed. No self-imposed requeue is returned; the controller's
// steady-state requeue drives re-entry and the annotation check gates writes.
func (r *StatusConfigMapReconciler) Name() string { return statusConfigMapReconcilerName }

func (r *StatusConfigMapReconciler) Reconcile() (ctrl.Result, error) {

	if !r.instance.Spec.General.StatusConfigMap.Enable {
		// StateAbsent deletes the ConfigMap if present and is a no-op otherwise.
		cm := r.buildConfigMap("", "")
		if _, err := r.client.ReconcileResource(cm, reconciler.StateAbsent); err != nil {
			r.recorder.Event(r.instance, "Warning", opensearchError, "error deleting status ConfigMap")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	interval := 60 * time.Second
	if r.instance.Spec.General.StatusConfigMap.UpdateInterval != nil {
		interval = r.instance.Spec.General.StatusConfigMap.UpdateInterval.Duration
	}

	if r.withinUpdateInterval(interval) {
		return ctrl.Result{}, nil
	}

	r.logger.Info("starting status configmap reconcile", "phase", r.instance.Status.Phase)

	var retErr error

	r.osClient, retErr = util.CreateClientForCluster(r.client, r.ctx, r.instance, r.osClientTransport)
	if retErr != nil {
		r.recorder.Event(r.instance, "Warning", opensearchError, "error creating opensearch client")
		return ctrl.Result{}, retErr
	}

	clusterData, clusterDataErr := util.GatherClusterData(r.osClient)
	diagnostics := util.BuildStatusText(r.instance.Name, *clusterData, *clusterDataErr)
	problems := util.BuildProblemsText(r.instance.Name, *clusterData, *clusterDataErr)

	cm := r.buildConfigMap(diagnostics, problems)
	if err := ctrl.SetControllerReference(r.instance, cm, r.client.Scheme()); err != nil {
		return ctrl.Result{}, err
	}
	result, retErr := r.client.CreateConfigMap(cm)
	if retErr != nil {
		r.recorder.Event(r.instance, "Warning", opensearchError, "error reconciling status ConfigMap")
		if result != nil {
			return *result, retErr
		}
		return ctrl.Result{}, retErr
	}

	return ctrl.Result{}, nil
}

// withinUpdateInterval reports whether the last write (stored as an annotation
// on the ConfigMap) is more recent than interval. Any missing, unparseable, or
// error case returns false, causing a refresh at worst.
func (r *StatusConfigMapReconciler) withinUpdateInterval(interval time.Duration) bool {
	cmName := fmt.Sprintf("%s-status", r.instance.Name)
	existing, err := r.client.GetConfigMap(cmName, r.instance.Namespace)
	if err != nil {
		if !errors.IsNotFound(err) {
			r.logger.V(1).Info("could not read status ConfigMap annotation, will refresh", "err", err)
		}
		return false
	}
	lastStr, ok := existing.Annotations[statusConfigMapLastUpdateAnnotation]
	if !ok {
		return false
	}
	last, err := time.Parse(time.RFC3339, lastStr)
	if err != nil {
		return false
	}
	return time.Since(last) < interval
}

// buildConfigMap constructs the status ConfigMap. The last-updated annotation
// is set to now so the next reconcile can skip writes within the interval.
func (r *StatusConfigMapReconciler) buildConfigMap(diagnostics, problems string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-status", r.instance.Name),
			Namespace: r.instance.Namespace,
			Annotations: map[string]string{
				statusConfigMapLastUpdateAnnotation: time.Now().UTC().Format(time.RFC3339),
			},
		},
		Data: map[string]string{
			"diagnostics": diagnostics,
			"problems":    problems,
		},
	}
}
