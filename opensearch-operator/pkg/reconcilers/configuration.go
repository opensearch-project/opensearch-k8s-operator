package reconcilers

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	"github.com/cisco-open/operator-tools/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ConfigurationReconciler struct {
	client            k8s.K8sClient
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
		client:            k8s.NewK8sClient(client, ctx, append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "configuration")))...),
		reconcilerContext: reconcilerContext,
		recorder:          recorder,
		instance:          instance,
	}
}

func (r *ConfigurationReconciler) Reconcile() (ctrl.Result, error) {
	// Create emptyDir volumes for writable directories that OpenSearch needs
	writableVolumes := []opsterv1.AdditionalVolume{
		{
			Name:     "rw-conf",
			Path:     "/usr/share/opensearch/conf",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
		{
			Name:     "rw-logs",
			Path:     "/usr/share/opensearch/logs",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}

	// Add plugins directory volume if pluginsList is not empty
	if len(r.instance.Spec.General.PluginsList) > 0 {
		writableVolumes = append(writableVolumes, opsterv1.AdditionalVolume{
			Name:     "rw-plugins",
			Path:     "/usr/share/opensearch/plugins",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		})
	}

	// Create the volumes and mounts
	volumes, volumeMounts, _, err := util.CreateAdditionalVolumes(
		r.client,
		r.instance.Namespace,
		writableVolumes,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Add the volumes and mounts to the reconciler context
	r.reconcilerContext.Volumes = append(r.reconcilerContext.Volumes, volumes...)
	r.reconcilerContext.VolumeMounts = append(r.reconcilerContext.VolumeMounts, volumeMounts...)

	if len(r.instance.Spec.General.AdditionalVolumes) == 0 &&
		(r.reconcilerContext.OpenSearchConfig == nil || len(r.reconcilerContext.OpenSearchConfig) == 0) {
		return ctrl.Result{}, nil
	}
	systemIndices, err := json.Marshal(services.AdditionalSystemIndices)
	if err != nil {
		return ctrl.Result{}, err
	}

	if len(r.reconcilerContext.OpenSearchConfig) > 0 {
		// Add some default config for the security plugin
		r.reconcilerContext.AddConfig("plugins.security.audit.type", "internal_opensearch")
		r.reconcilerContext.AddConfig("plugins.security.enable_snapshot_restore_privilege", "true")
		r.reconcilerContext.AddConfig("plugins.security.check_snapshot_restore_write_privileges", "true")
		r.reconcilerContext.AddConfig("plugins.security.restapi.roles_enabled", `["all_access", "security_rest_api_access"]`)
		r.reconcilerContext.AddConfig("plugins.security.system_indices.enabled", "true")
		r.reconcilerContext.AddConfig("plugins.security.system_indices.indices", string(systemIndices))

	}

	var sb strings.Builder
	keys := make([]string, 0, len(r.reconcilerContext.OpenSearchConfig))
	for key := range r.reconcilerContext.OpenSearchConfig {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		sb.WriteString(fmt.Sprintf("%s: %s\n", key, r.reconcilerContext.OpenSearchConfig[key]))
	}
	data := sb.String()
	result := reconciler.CombinedResult{}

	if r.reconcilerContext.OpenSearchConfig != nil && len(r.reconcilerContext.OpenSearchConfig) != 0 {
		cm := r.buildConfigMap(data)
		if err := ctrl.SetControllerReference(r.instance, cm, r.client.Scheme()); err != nil {
			return ctrl.Result{}, err
		}

		result.Combine(r.client.CreateConfigMap(cm))
		if result.Err != nil {
			return result.Result, result.Err
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
	}

	// Generate additional volumes
	addVolumes, addVolumeMounts, addVolumeData, err := util.CreateAdditionalVolumes(
		r.client,
		r.instance.Namespace,
		r.instance.Spec.General.AdditionalVolumes,
	)
	if err != nil {
		result.CombineErr(err)
		return result.Result, result.Err
	}

	r.reconcilerContext.Volumes = append(r.reconcilerContext.Volumes, addVolumes...)
	r.reconcilerContext.VolumeMounts = append(r.reconcilerContext.VolumeMounts, addVolumeMounts...)

	for _, nodePool := range r.instance.Spec.NodePools {
		result.Combine(r.createHashForNodePool(nodePool, data, addVolumeData))
	}

	return result.Result, result.Err
}

func (r *ConfigurationReconciler) buildConfigMap(data string) *corev1.ConfigMap {
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

func (r *ConfigurationReconciler) createHashForNodePool(nodePool opsterv1.NodePool, data string, volumeData []byte) (*ctrl.Result, error) {
	combinedData := append([]byte(data), volumeData...)

	found, nodePoolHash := r.reconcilerContext.fetchNodePoolHash(nodePool.Component)
	// If we don't find the NodePoolConfig this indicates there's been an update to the CR
	// since starting reconciliation so we requeue
	if !found {
		return &ctrl.Result{
			Requeue: true,
		}, nil
	}

	// If an upgrade is in process we want to wait to schedule non data nodes
	// data nodes will be picked up by the rolling restarter, or the upgrade
	if r.instance.Status.Version != "" && r.instance.Status.Version != r.instance.Spec.General.Version {
		if !helpers.HasDataRole(&nodePool) {
			sts, err := r.client.GetStatefulSet(builders.StsName(r.instance, &nodePool), r.instance.Namespace)
			if k8serrors.IsNotFound(err) {
				nodePoolHash.ConfigHash = generateHash(combinedData)
			} else if err != nil {
				return nil, err
			} else {
				nodePoolHash.ConfigHash = sts.Spec.Template.Annotations[builders.ConfigurationChecksumAnnotation]
			}
		}
	} else {
		nodePoolHash.ConfigHash = generateHash(combinedData)
	}

	r.reconcilerContext.replaceNodePoolHash(nodePoolHash)
	return nil, nil
}

func (r *ConfigurationReconciler) DeleteResources() (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func generateHash(source []byte) string {
	hash := sha1.New()
	hash.Write(source)
	return hex.EncodeToString(hash.Sum(nil))
}
