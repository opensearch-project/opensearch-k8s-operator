package util

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/metrics"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/tls"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kube-openapi/pkg/validation/errors"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	CaCertKey = "ca.crt"
)

func CheckEquels(from_env *appsv1.StatefulSetSpec, from_crd *appsv1.StatefulSetSpec, text string) (int32, bool, error) {
	field_env := helpers.GetField(from_env, text)
	field_env_int_ptr, ok := field_env.(*int32)
	if !ok {
		err := errors.New(777, "something was worng")
		return *field_env_int_ptr, false, err
	}
	if field_env_int_ptr == nil {
		err := errors.New(777, "something was worng")
		return *field_env_int_ptr, false, err

	}
	field_crd := helpers.GetField(from_crd, "Replicas")
	field_crd_int_ptr, ok := field_crd.(*int32)
	if !ok {
		err := errors.New(777, "something was worng")
		return *field_crd_int_ptr, false, err
	}
	if field_crd_int_ptr == nil {
		err := errors.New(777, "something was worng")
		return *field_crd_int_ptr, false, err

	}

	if field_env_int_ptr != field_crd_int_ptr {
		return *field_crd_int_ptr, false, nil
	} else {
		return *field_crd_int_ptr, true, nil
	}
}

func ReadOrGenerateCaCert(pki tls.PKI, k8sClient k8s.K8sClient, instance *opsterv1.OpenSearchCluster) (tls.Cert, error) {
	namespace := instance.Namespace
	clusterName := instance.Name
	secretName := clusterName + "-ca"
	logger := log.FromContext(k8sClient.Context())
	var ca tls.Cert
	caSecret, err := k8sClient.GetSecret(secretName, namespace)
	if err != nil {
		// Generate CA cert and put it into secret
		ca, err = pki.GenerateCA(clusterName)
		if err != nil {
			logger.Error(err, "Failed to create CA")
			return ca, err
		}
		caSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace}, Data: ca.SecretDataCA()}
		if err := ctrl.SetControllerReference(instance, &caSecret, k8sClient.Scheme()); err != nil {
			return ca, err
		}
		if _, err := k8sClient.CreateSecret(&caSecret); err != nil {
			logger.Error(err, "Failed to store CA in secret")
			return ca, err
		}
	} else {
		ca = pki.CAFromSecret(caSecret.Data)
	}

	validator, err := tls.NewCertValidater(ca.CertData())
	if err != nil {
		return ca, err
	}
	metrics.TLSCertExpiryDays.WithLabelValues(clusterName, namespace, CaCertKey).Set(validator.DaysUntilExpiry())

	return ca, nil
}

func CreateAdditionalVolumes(
	k8sClient k8s.K8sClient,
	namespace string,
	volumeConfigs []opsterv1.AdditionalVolume,
) (
	retVolumes []corev1.Volume,
	retVolumeMounts []corev1.VolumeMount,
	retData []byte,
	returnErr error,
) {
	lg := log.FromContext(k8sClient.Context())
	var names []string
	namesIndex := map[string]int{}

	for i, volumeConfig := range volumeConfigs {
		readOnly := true
		if volumeConfig.ConfigMap != nil {
			retVolumes = append(retVolumes, corev1.Volume{
				Name: volumeConfig.Name,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: volumeConfig.ConfigMap,
				},
			})
		}
		if volumeConfig.Secret != nil {
			retVolumes = append(retVolumes, corev1.Volume{
				Name: volumeConfig.Name,
				VolumeSource: corev1.VolumeSource{
					Secret: volumeConfig.Secret,
				},
			})
		}
		if volumeConfig.EmptyDir != nil {
			readOnly = false
			retVolumes = append(retVolumes, corev1.Volume{
				Name: volumeConfig.Name,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: volumeConfig.EmptyDir,
				},
			})
		}
		if volumeConfig.CSI != nil {
			if volumeConfig.CSI.ReadOnly != nil {
				readOnly = *volumeConfig.CSI.ReadOnly
			}
			retVolumes = append(retVolumes, corev1.Volume{
				Name: volumeConfig.Name,
				VolumeSource: corev1.VolumeSource{
					CSI: volumeConfig.CSI,
				},
			})
		}
		if volumeConfig.Projected != nil {
			retVolumes = append(retVolumes, corev1.Volume{
				Name: volumeConfig.Name,
				VolumeSource: corev1.VolumeSource{
					Projected: volumeConfig.Projected,
				},
			})
		}
		if volumeConfig.RestartPods {
			namesIndex[volumeConfig.Name] = i
			names = append(names, volumeConfig.Name)
		}

		subPath := ""
		// SubPaths are only supported for ConfigMaps, Secrets and CSI volumes
		if volumeConfig.ConfigMap != nil || volumeConfig.Secret != nil || volumeConfig.CSI != nil || volumeConfig.Projected != nil {
			subPath = strings.TrimSpace(volumeConfig.SubPath)
		}

		retVolumeMounts = append(retVolumeMounts, corev1.VolumeMount{
			Name:      volumeConfig.Name,
			ReadOnly:  readOnly,
			MountPath: volumeConfig.Path,
			SubPath:   subPath,
		})
	}
	sort.Strings(names)

	for _, name := range names {
		volumeConfig := volumeConfigs[namesIndex[name]]
		if volumeConfig.ConfigMap != nil {
			cm, err := k8sClient.GetConfigMap(volumeConfig.ConfigMap.Name, namespace)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					lg.V(1).Error(err, "failed to find configMap for additional volume")
					continue
				}
				returnErr = err
				return
			}
			data := cm.Data
			keys := make([]string, 0, len(data))
			for key := range data {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				retData = append(retData, []byte(data[key])...)
			}
		}

		if volumeConfig.Secret != nil {
			secret, err := k8sClient.GetSecret(volumeConfig.Secret.SecretName, namespace)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					lg.V(1).Error(err, "failed to find secret for additional volume")
					continue
				}
				returnErr = err
				return
			}
			data := secret.Data
			keys := make([]string, 0, len(data))
			for key := range data {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				retData = append(retData, data[key]...)
			}
		}
	}

	return
}

func OpensearchClusterURL(cluster *opsterv1.OpenSearchCluster) string {
	return fmt.Sprintf(
		"https://%s.%s.svc.%s:%v",
		cluster.Spec.General.ServiceName,
		cluster.Namespace,
		helpers.ClusterDnsBase(),
		cluster.Spec.General.HttpPort,
	)
}

func CreateClientForCluster(
	k8sClient k8s.K8sClient,
	ctx context.Context,
	cluster *opsterv1.OpenSearchCluster,
	transport http.RoundTripper,
) (*services.OsClusterClient, error) {
	lg := log.FromContext(ctx)
	var osClient *services.OsClusterClient

	username, password, err := helpers.UsernameAndPassword(k8sClient, cluster)
	if err != nil {
		lg.Error(err, "failed to fetch opensearch credentials")
		return nil, err
	}

	if transport == nil {
		osClient, err = services.NewOsClusterClient(
			OpensearchClusterURL(cluster),
			username,
			password,
		)
	} else {
		osClient, err = services.NewOsClusterClient(
			OpensearchClusterURL(cluster),
			username,
			password,
			services.WithTransport(transport),
		)
	}

	return osClient, err
}

func FetchOpensearchCluster(
	k8sClient k8s.K8sClient,
	ctx context.Context,
	ref types.NamespacedName,
) (*opsterv1.OpenSearchCluster, error) {
	cluster, err := k8sClient.GetOpenSearchCluster(ref.Name, ref.Namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &cluster, nil
}

// Generates a checksum of binary data using the SHA1 algorithm.
func GetSha1Sum(data []byte) (string, error) {
	hasher := sha1.New()
	_, err := hasher.Write(data)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func DataNodesCount(k8sClient k8s.K8sClient, cr *opsterv1.OpenSearchCluster) int32 {
	count := int32(0)
	for _, nodePool := range cr.Spec.NodePools {
		if helpers.HasDataRole(&nodePool) {
			sts, err := k8sClient.GetStatefulSet(builders.StsName(cr, &nodePool), cr.Namespace)
			if err == nil {
				count = count + pointer.Int32Deref(sts.Spec.Replicas, 1)
			}
		}
	}
	return count
}

// GetClusterHealth returns the health of OpenSearch cluster
func GetClusterHealth(k8sClient k8s.K8sClient, ctx context.Context, cluster *opsterv1.OpenSearchCluster, lg logr.Logger) opsterv1.OpenSearchHealth {
	osClient, err := CreateClientForCluster(k8sClient, ctx, cluster, nil)
	if err != nil {
		lg.V(1).Info(fmt.Sprintf("Failed to create OS client while checking cluster health: %v", err))
		return opsterv1.OpenSearchUnknownHealth
	}

	healthResponse, err := osClient.GetClusterHealth()
	if err != nil {
		lg.Error(err, "Failed to get OpenSearch health status")
		return opsterv1.OpenSearchUnknownHealth
	}

	return opsterv1.OpenSearchHealth(healthResponse.Status)
}

// GetAvailableOpenSearchNodes returns the sum of ready pods for all node pools
func GetAvailableOpenSearchNodes(k8sClient k8s.K8sClient, ctx context.Context, cluster *opsterv1.OpenSearchCluster, lg logr.Logger) int32 {
	clusterName := cluster.Name
	clusterNamespace := cluster.Namespace

	previousAvailableNodes := cluster.Status.AvailableNodes
	var availableNodes int32

	for _, nodePool := range cluster.Spec.NodePools {
		var sts *appsv1.StatefulSet
		var err error

		sts, err = helpers.GetSTSForNodePool(k8sClient, nodePool, clusterName, clusterNamespace)
		if err != nil {
			lg.V(1).Info(fmt.Sprintf("Failed to get statefulsets for nodepool %s: %v", nodePool.Component, err))
			return previousAvailableNodes
		}

		if sts != nil {
			availableNodes += sts.Status.ReadyReplicas
		}
	}

	return availableNodes
}
