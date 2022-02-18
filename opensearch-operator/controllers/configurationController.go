package controllers

import (
	"context"
	"fmt"
	"strings"

	//v1 "k8s.io/client-go/applyconfigurations/core/v1"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigurationReconciler struct {
	client.Client
	Recorder record.EventRecorder
	logr.Logger
	Instance *opsterv1.OpenSearchCluster
}

func (r *ConfigurationReconciler) Reconcile(controllerContext *ControllerContext) (*opsterv1.ComponentStatus, error) {
	if controllerContext.OpenSearchConfig == nil || len(controllerContext.OpenSearchConfig) == 0 {
		return nil, nil
	}
	namespace := r.Instance.Spec.General.ClusterName
	clusterName := r.Instance.Spec.General.ClusterName
	configMapName := clusterName + "-config"

	// Add some default config for the security plugin
	controllerContext.AddConfig("plugins.security.audit.type", "internal_opensearch")
	controllerContext.AddConfig("plugins.security.allow_default_init_securityindex", "true") // TODO: Remove after securityconfig is managed by controller
	controllerContext.AddConfig("plugins.security.enable_snapshot_restore_privilege", "true")
	controllerContext.AddConfig("plugins.security.check_snapshot_restore_write_privileges", "true")
	controllerContext.AddConfig("plugins.security.restapi.roles_enabled", `["all_access", "security_rest_api_access"]`)
	controllerContext.AddConfig("plugins.security.system_indices.enabled", "true")
	controllerContext.AddConfig("plugins.security.system_indices.indices", `[".opendistro-alerting-config", ".opendistro-alerting-alert*", ".opendistro-anomaly-results*", ".opendistro-anomaly-detector*", ".opendistro-anomaly-checkpoints", ".opendistro-anomaly-detection-state", ".opendistro-reports-*", ".opendistro-notifications-*", ".opendistro-notebooks", ".opensearch-observability", ".opendistro-asynchronous-search-response*", ".replication-metadata-store"]`)

	cm := corev1.ConfigMap{}
	// TODO: Update if exists
	if err := r.Client.Get(context.TODO(), client.ObjectKey{Name: configMapName, Namespace: namespace}, &cm); err != nil {
		var sb strings.Builder
		for key, value := range controllerContext.OpenSearchConfig {
			sb.WriteString(fmt.Sprintf("%s: %s\n", key, value))
		}
		data := sb.String()
		cm = corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: namespace,
			},
			Data: map[string]string{
				"opensearch.yml": data,
			},
		}
		err = r.Create(context.TODO(), &cm)
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				r.Logger.Error(err, "Cannot create Configmap "+configMapName)
				r.Recorder.Event(r.Instance, "Warning", "Cannot create Configmap ", "Requeue - Fix the problem you have on main Opensearch ConfigMap")
				return nil, err
			}
		}
	}
	volume := corev1.Volume{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: configMapName}}}}
	controllerContext.Volumes = append(controllerContext.Volumes, volume)
	mount := corev1.VolumeMount{Name: "config", MountPath: "/usr/share/opensearch/config/opensearch.yml", SubPath: "opensearch.yml"}
	controllerContext.VolumeMounts = append(controllerContext.VolumeMounts, mount)

	return nil, nil
}
