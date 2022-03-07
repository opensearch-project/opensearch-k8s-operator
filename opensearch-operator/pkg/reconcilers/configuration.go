package reconcilers

import (
	"context"
	"fmt"
	"strings"

	"github.com/banzaicloud/operator-tools/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ConfigurationReconciler struct {
	reconciler.ResourceReconciler
	client.Client
	ctx               context.Context
	recorder          record.EventRecorder
	reconcilerContext *ReconcilerContext
	instance          *opsterv1.OpenSearchCluster
}

func NewConfigurationReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	reconcilerContext *ReconcilerContext,
	instance *opsterv1.OpenSearchCluster,
	opts ...reconciler.ResourceReconcilerOption,
) *ConfigurationReconciler {
	return &ConfigurationReconciler{
		Client: client,
		ResourceReconciler: reconciler.NewReconcilerWith(client,
			append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "configuration")))...),
		ctx:               ctx,
		reconcilerContext: reconcilerContext,
		recorder:          recorder,
		instance:          instance,
	}
}

func (r *ConfigurationReconciler) Reconcile() (ctrl.Result, error) {
	if r.reconcilerContext.OpenSearchConfig == nil || len(r.reconcilerContext.OpenSearchConfig) == 0 {
		return ctrl.Result{}, nil
	}
	// Add some default config for the security plugin
	r.reconcilerContext.AddConfig("plugins.security.audit.type", "internal_opensearch")
	r.reconcilerContext.AddConfig("plugins.security.allow_default_init_securityindex", "true") // TODO: Remove after securityconfig is managed by controller
	r.reconcilerContext.AddConfig("plugins.security.enable_snapshot_restore_privilege", "true")
	r.reconcilerContext.AddConfig("plugins.security.check_snapshot_restore_write_privileges", "true")
	r.reconcilerContext.AddConfig("plugins.security.restapi.roles_enabled", `["all_access", "security_rest_api_access"]`)
	r.reconcilerContext.AddConfig("plugins.security.system_indices.enabled", "true")
	r.reconcilerContext.AddConfig("plugins.security.system_indices.indices", `[".opendistro-alerting-config", ".opendistro-alerting-alert*", ".opendistro-anomaly-results*", ".opendistro-anomaly-detector*", ".opendistro-anomaly-checkpoints", ".opendistro-anomaly-detection-state", ".opendistro-reports-*", ".opendistro-notifications-*", ".opendistro-notebooks", ".opensearch-observability", ".opendistro-asynchronous-search-response*", ".replication-metadata-store"]`)

	cm := r.buildConfigMap()
	result, err := r.ReconcileResource(cm, reconciler.StatePresent)
	if err != nil {
		r.recorder.Event(r.instance, "Warning", "Cannot create Configmap ", "Requeue - Fix the problem you have on main Opensearch ConfigMap")
	}

	volume := corev1.Volume{
		Name: "config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cm.Name,
				},
			},
		},
	}
	r.reconcilerContext.Volumes = append(r.reconcilerContext.Volumes, volume)

	mount := corev1.VolumeMount{
		Name:      "config",
		MountPath: "/usr/share/opensearch/config/opensearch.yml",
		SubPath:   "opensearch.yml",
	}
	r.reconcilerContext.VolumeMounts = append(r.reconcilerContext.VolumeMounts, mount)

	if result != nil {
		return *result, err
	}
	return ctrl.Result{}, err
}

func (r *ConfigurationReconciler) buildConfigMap() *corev1.ConfigMap {
	var sb strings.Builder
	for key, value := range r.reconcilerContext.OpenSearchConfig {
		sb.WriteString(fmt.Sprintf("%s: %s\n", key, value))
	}
	data := sb.String()

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-config", r.instance.Name),
			Namespace: r.instance.Namespace,
		},
		Data: map[string]string{
			"opensearch.yml": data,
		},
	}
}

func (r *ConfigurationReconciler) DeleteResources() (ctrl.Result, error) {
	cm := r.buildConfigMap()
	_, err := r.ReconcileResource(cm, reconciler.StateAbsent)
	return ctrl.Result{}, err
}
