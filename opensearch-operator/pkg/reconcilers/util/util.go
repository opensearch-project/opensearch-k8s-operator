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
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/tls"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kube-openapi/pkg/validation/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

func ReadOrGenerateCaCert(pki tls.PKI, k8sClient client.Client, ctx context.Context, instance *opsterv1.OpenSearchCluster) (tls.Cert, error) {
	namespace := instance.Namespace
	clusterName := instance.Name
	secretName := clusterName + "-ca"
	logger := log.FromContext(ctx)
	caSecret := corev1.Secret{}
	var ca tls.Cert
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: secretName, Namespace: namespace}, &caSecret); err != nil {
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
		if err := k8sClient.Create(ctx, &caSecret); err != nil {
			logger.Error(err, "Failed to store CA in secret")
			return ca, err
		}
	} else {
		ca = pki.CAFromSecret(caSecret.Data)
	}
	return ca, nil
}

func CreateAdditionalVolumes(
	ctx context.Context,
	k8sClient client.Client,
	namespace string,
	volumeConfigs []opsterv1.AdditionalVolume,
) (
	retVolumes []corev1.Volume,
	retVolumeMounts []corev1.VolumeMount,
	retData []byte,
	returnErr error,
) {
	lg := log.FromContext(ctx)
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
		if volumeConfig.RestartPods {
			namesIndex[volumeConfig.Name] = i
			names = append(names, volumeConfig.Name)
		}

		subPath := ""
		// SubPaths are only supported for ConfigMaps and Secrets
		if volumeConfig.ConfigMap != nil || volumeConfig.Secret != nil {
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
			cm := &corev1.ConfigMap{}
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      volumeConfig.ConfigMap.Name,
				Namespace: namespace,
			}, cm); err != nil {
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
			secret := &corev1.Secret{}
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      volumeConfig.Secret.SecretName,
				Namespace: namespace,
			}, secret); err != nil {
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
	ctx context.Context,
	k8sClient client.Client,
	cluster *opsterv1.OpenSearchCluster,
	transport http.RoundTripper,
) (*services.OsClusterClient, error) {
	lg := log.FromContext(ctx)
	var osClient *services.OsClusterClient

	username, password, err := helpers.UsernameAndPassword(ctx, k8sClient, cluster)
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
	ctx context.Context,
	k8sClient client.Client,
	ref types.NamespacedName,
) (*opsterv1.OpenSearchCluster, error) {
	cluster := &opsterv1.OpenSearchCluster{}
	err := k8sClient.Get(ctx, ref, cluster)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return cluster, nil
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

// GetClusterHealth returns the health of OpenSearch cluster
func GetClusterHealth(ctx context.Context, k8sClient client.Client, cluster *opsterv1.OpenSearchCluster, lg logr.Logger) opsterv1.OpenSearchHealth {
	osClient, err := CreateClientForCluster(ctx, k8sClient, cluster, nil)
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
func GetAvailableOpenSearchNodes(ctx context.Context, k8sClient client.Client, cluster *opsterv1.OpenSearchCluster, lg logr.Logger) int32 {
	clusterName := cluster.Name
	clusterNamespace := cluster.Namespace

	previousAvailableNodes := cluster.Status.AvailableNodes
	var availableNodes int32

	for _, nodePool := range cluster.Spec.NodePools {
		var sts *appsv1.StatefulSet
		var err error

		sts, err = helpers.GetSTSForNodePool(ctx, k8sClient, nodePool, clusterName, clusterNamespace)
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
