package reconcilers

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconciler"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	corev1 "k8s.io/api/core/v1"
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
	instance          *opensearchv1.OpenSearchCluster
}

func NewConfigurationReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	reconcilerContext *ReconcilerContext,
	instance *opensearchv1.OpenSearchCluster,
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
	// Check if we have any config to process
	hasGeneralConfig := len(r.instance.Spec.General.AdditionalConfig) > 0
	hasNodePoolConfig := false
	for _, nodePool := range r.instance.Spec.NodePools {
		if len(nodePool.AdditionalConfig) > 0 {
			hasNodePoolConfig = true
			break
		}
	}

	if len(r.instance.Spec.General.AdditionalVolumes) == 0 &&
		len(r.reconcilerContext.OpenSearchConfig) == 0 &&
		!hasGeneralConfig && !hasNodePoolConfig {
		return ctrl.Result{}, nil
	}
	systemIndices, err := json.Marshal(services.AdditionalSystemIndices)
	if err != nil {
		return ctrl.Result{}, err
	}

	if helpers.IsSecurityPluginEnabled(r.instance) {
		// Add some default config for the security plugin
		r.reconcilerContext.AddConfig("plugins.security.audit.type", "internal_opensearch")
		r.reconcilerContext.AddConfig("plugins.security.enable_snapshot_restore_privilege", "true")
		r.reconcilerContext.AddConfig("plugins.security.check_snapshot_restore_write_privileges", "true")
		r.reconcilerContext.AddConfig("plugins.security.restapi.roles_enabled", `["all_access", "security_rest_api_access"]`)
		r.reconcilerContext.AddConfig("plugins.security.system_indices.enabled", "true")
		r.reconcilerContext.AddConfig("plugins.security.system_indices.indices", string(systemIndices))
	}

	// Process gRPC configuration
	r.processGrpcConfig()

	// Add General.AdditionalConfig to reconciler context (for base config)
	for k, v := range r.instance.Spec.General.AdditionalConfig {
		r.reconcilerContext.AddConfig(k, v)
	}

	// Helper function to build config string from a map
	buildConfigString := func(config map[string]string) string {
		var sb strings.Builder
		keys := make([]string, 0, len(config))
		for key := range config {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			sb.WriteString(fmt.Sprintf("%s: %s\n", key, config[key]))
		}
		return sb.String()
	}

	result := reconciler.CombinedResult{}

	// Always create shared configmap if General.AdditionalConfig exists (for bootstrap and security update jobs)
	// This is needed even when per-nodepool configmaps are created
	if len(r.reconcilerContext.OpenSearchConfig) != 0 {
		baseData := buildConfigString(r.reconcilerContext.OpenSearchConfig)
		cm := r.buildConfigMap(baseData)
		if err := ctrl.SetControllerReference(r.instance, cm, r.client.Scheme()); err != nil {
			return ctrl.Result{}, err
		}

		result.Combine(r.client.CreateConfigMap(cm))
		if result.Err != nil {
			return result.Result, result.Err
		}

		// Add shared volume and mount for shared configmap (used by bootstrap and security update jobs)
		// Nodepools with AdditionalConfig will override this with their own per-nodepool configmap
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

	// Create per-nodepool configmaps only for nodepools that have AdditionalConfig
	for _, nodePool := range r.instance.Spec.NodePools {
		if len(nodePool.AdditionalConfig) > 0 {
			// Start with base config (system configs + General.AdditionalConfig)
			mergedConfig := make(map[string]string)
			for k, v := range r.reconcilerContext.OpenSearchConfig {
				mergedConfig[k] = v
			}
			// Merge NodePool.AdditionalConfig (overrides General.AdditionalConfig)
			for k, v := range nodePool.AdditionalConfig {
				mergedConfig[k] = v
			}

			nodePoolData := buildConfigString(mergedConfig)
			cmName := fmt.Sprintf("%s-%s-config", r.instance.Name, nodePool.Component)
			cm := r.buildConfigMapForNodePool(nodePoolData, cmName)
			if err := ctrl.SetControllerReference(r.instance, cm, r.client.Scheme()); err != nil {
				return ctrl.Result{}, err
			}

			result.Combine(r.client.CreateConfigMap(cm))
			if result.Err != nil {
				return result.Result, result.Err
			}
		}
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

	r.reconcilerContext.Volumes = append(addVolumes, r.reconcilerContext.Volumes...)
	r.reconcilerContext.VolumeMounts = append(addVolumeMounts, r.reconcilerContext.VolumeMounts...)

	for _, nodePool := range r.instance.Spec.NodePools {
		// Build merged config for hash calculation
		mergedConfig := make(map[string]string)
		for k, v := range r.reconcilerContext.OpenSearchConfig {
			mergedConfig[k] = v
		}
		// Merge NodePool.AdditionalConfig (overrides General.AdditionalConfig)
		for k, v := range nodePool.AdditionalConfig {
			mergedConfig[k] = v
		}
		dataToUse := buildConfigString(mergedConfig)
		result.Combine(r.createHashForNodePool(nodePool, dataToUse, addVolumeData))
	}

	return result.Result, result.Err
}

func (r *ConfigurationReconciler) buildConfigMap(data string) *corev1.ConfigMap {
	return r.buildConfigMapForNodePool(data, fmt.Sprintf("%s-config", r.instance.Name))
}

func (r *ConfigurationReconciler) buildConfigMapForNodePool(data string, name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.instance.Namespace,
		},
		Data: map[string]string{
			"opensearch.yml": data,
		},
	}
}

func (r *ConfigurationReconciler) createHashForNodePool(nodePool opensearchv1.NodePool, data string, volumeData []byte) (*ctrl.Result, error) {
	combinedData := append([]byte(data), volumeData...)

	found, nodePoolHash := r.reconcilerContext.fetchNodePoolHash(nodePool.Component)
	// If we don't find the NodePoolConfig this indicates there's been an update to the CR
	// since starting reconciliation so we requeue
	if !found {
		return &ctrl.Result{
			Requeue: true,
		}, nil
	}

	// Calculate the hash for all node pools, including during upgrade.
	// This ensures the config hash annotation is set correctly during upgrade
	// and won't need to be updated after upgrade completes, preventing unnecessary
	// StatefulSet revisions and rolling restarts.
	nodePoolHash.ConfigHash = generateHash(combinedData)

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

// processGrpcConfig processes gRPC configuration and adds it to opensearch.yml
func (r *ConfigurationReconciler) processGrpcConfig() {
	grpcConfig := r.instance.Spec.General.Grpc
	if grpcConfig == nil || !grpcConfig.Enable {
		return
	}

	// Set aux.transport.types to use transport-grpc
	r.reconcilerContext.AddConfig("aux.transport.types", "[transport-grpc]")

	// Set port if specified, otherwise use default
	port := grpcConfig.Port
	if port == "" {
		port = "9400-9500"
	}
	r.reconcilerContext.AddConfig("aux.transport.transport-grpc.port", fmt.Sprintf("'%s'", port))

	// Set host addresses
	if len(grpcConfig.Host) > 0 {
		hostList := make([]string, len(grpcConfig.Host))
		for i, h := range grpcConfig.Host {
			hostList[i] = fmt.Sprintf(`"%s"`, h)
		}
		r.reconcilerContext.AddConfig("grpc.host", fmt.Sprintf("[%s]", strings.Join(hostList, ", ")))
	}

	// Set bind host
	if len(grpcConfig.BindHost) > 0 {
		bindHostList := make([]string, len(grpcConfig.BindHost))
		for i, h := range grpcConfig.BindHost {
			bindHostList[i] = fmt.Sprintf(`"%s"`, h)
		}
		r.reconcilerContext.AddConfig("grpc.bind_host", fmt.Sprintf("[%s]", strings.Join(bindHostList, ", ")))
	}

	// Set publish host
	if len(grpcConfig.PublishHost) > 0 {
		publishHostList := make([]string, len(grpcConfig.PublishHost))
		for i, h := range grpcConfig.PublishHost {
			publishHostList[i] = fmt.Sprintf(`"%s"`, h)
		}
		r.reconcilerContext.AddConfig("grpc.publish_host", fmt.Sprintf("[%s]", strings.Join(publishHostList, ", ")))
	}

	// Set publish port
	if grpcConfig.PublishPort != nil {
		r.reconcilerContext.AddConfig("grpc.publish_port", fmt.Sprintf("%d", *grpcConfig.PublishPort))
	}

	// Set Netty worker count
	if grpcConfig.NettyWorkerCount != nil {
		r.reconcilerContext.AddConfig("grpc.netty.worker_count", fmt.Sprintf("%d", *grpcConfig.NettyWorkerCount))
	}

	// Set Netty executor count
	if grpcConfig.NettyExecutorCount != nil {
		r.reconcilerContext.AddConfig("grpc.netty.executor_count", fmt.Sprintf("%d", *grpcConfig.NettyExecutorCount))
	}

	// Set max concurrent connection calls
	if grpcConfig.MaxConcurrentConnectionCalls != nil {
		r.reconcilerContext.AddConfig("grpc.netty.max_concurrent_connection_calls", fmt.Sprintf("%d", *grpcConfig.MaxConcurrentConnectionCalls))
	}

	// Set max connection age
	if grpcConfig.MaxConnectionAge != "" {
		r.reconcilerContext.AddConfig("grpc.netty.max_connection_age", grpcConfig.MaxConnectionAge)
	}

	// Set max connection idle
	if grpcConfig.MaxConnectionIdle != "" {
		r.reconcilerContext.AddConfig("grpc.netty.max_connection_idle", grpcConfig.MaxConnectionIdle)
	}

	// Set keepalive timeout
	if grpcConfig.KeepaliveTimeout != "" {
		r.reconcilerContext.AddConfig("grpc.netty.keepalive_timeout", grpcConfig.KeepaliveTimeout)
	}

	// Set max message size
	if grpcConfig.MaxMsgSize != "" {
		r.reconcilerContext.AddConfig("grpc.netty.max_msg_size", grpcConfig.MaxMsgSize)
	}
}
