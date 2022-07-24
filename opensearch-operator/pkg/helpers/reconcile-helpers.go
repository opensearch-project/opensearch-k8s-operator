package helpers

import (
	"context"
	"fmt"
	"path"
	"sort"

	"github.com/hashicorp/go-version"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kube-openapi/pkg/validation/errors"
	"k8s.io/utils/pointer"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/tls"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func CheckEquels(from_env *appsv1.StatefulSetSpec, from_crd *appsv1.StatefulSetSpec, text string) (int32, bool, error) {
	field_env := GetField(from_env, text)
	field_env_int_ptr, ok := field_env.(*int32)
	if !ok {
		err := errors.New(777, "something was worng")
		return *field_env_int_ptr, false, err
	}
	if field_env_int_ptr == nil {
		err := errors.New(777, "something was worng")
		return *field_env_int_ptr, false, err

	}
	field_crd := GetField(from_crd, "Replicas")
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

func ResolveImage(cr *opsterv1.OpenSearchCluster, nodePool *opsterv1.NodePool) (result opsterv1.ImageSpec) {
	defaultRepo := "docker.io/opensearchproject"
	defaultImage := "opensearch"

	var version string

	// If a general custom image is specified, use it.
	if cr.Spec.General.ImageSpec != nil {
		if useCustomImage(cr.Spec.General.ImageSpec, &result) {
			return
		}
	}

	// Calculate version based on upgrading status
	if nodePool == nil {
		version = cr.Spec.General.Version
	} else {
		componentStatus := opsterv1.ComponentStatus{
			Component:   "Upgrader",
			Description: nodePool.Component,
		}
		_, found := FindFirstPartial(cr.Status.ComponentsStatus, componentStatus, GetByDescriptionAndGroup)

		if cr.Status.Version == "" || cr.Status.Version == cr.Spec.General.Version {
			version = cr.Spec.General.Version
		} else {
			if found {
				version = cr.Spec.General.Version
			} else {
				version = cr.Status.Version
			}
		}
	}

	// If a different image repo is requested, use that with the default image
	// name and version tag.
	if cr.Spec.General.DefaultRepo != nil {
		defaultRepo = *cr.Spec.General.DefaultRepo
	}

	result.Image = pointer.String(fmt.Sprintf("%s:%s",
		path.Join(defaultRepo, defaultImage), version))
	return
}

func ResolveDashboardsImage(cr *opsterv1.OpenSearchCluster) (result opsterv1.ImageSpec) {
	defaultRepo := "docker.io/opensearchproject"
	defaultImage := "opensearch-dashboards"

	// If a custom dashboard image is specified, use it.
	if cr.Spec.Dashboards.ImageSpec != nil {
		if useCustomImage(cr.Spec.Dashboards.ImageSpec, &result) {
			return
		}
	}

	// If a general custom image is specified, use it.
	if cr.Spec.General.ImageSpec != nil {
		if useCustomImage(cr.Spec.General.ImageSpec, &result) {
			return
		}
	}

	// If a different image repo is requested, use that with the default image
	// name and version tag.
	if cr.Spec.General.DefaultRepo != nil {
		defaultRepo = *cr.Spec.General.DefaultRepo
	}

	result.Image = pointer.String(fmt.Sprintf("%s:%s",
		path.Join(defaultRepo, defaultImage), cr.Spec.Dashboards.Version))
	return
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
		if volumeConfig.RestartPods {
			namesIndex[volumeConfig.Name] = i
			names = append(names, volumeConfig.Name)
		}
		retVolumeMounts = append(retVolumeMounts, corev1.VolumeMount{
			Name:      volumeConfig.Name,
			ReadOnly:  true,
			MountPath: volumeConfig.Path,
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

func useCustomImage(customImageSpec *opsterv1.ImageSpec, result *opsterv1.ImageSpec) bool {
	if customImageSpec != nil {
		if customImageSpec.ImagePullPolicy != nil {
			result.ImagePullPolicy = customImageSpec.ImagePullPolicy
		}
		if len(customImageSpec.ImagePullSecrets) > 0 {
			result.ImagePullSecrets = customImageSpec.ImagePullSecrets
		}
		if customImageSpec.Image != nil {
			// If custom image is specified, use it.
			result.Image = customImageSpec.Image
			return true
		}
	}
	return false
}

//Function to help identify httpPort and securityconfigPath for 1.x and 2.x OpenSearch Operator.
func VersionCheck(instance *opsterv1.OpenSearchCluster) (int32, string) {
	var httpPort int32
	var securityconfigPath string
	versionPassed, _ := version.NewVersion(instance.Spec.General.Version)
	constraints, _ := version.NewConstraint(">= 2.0")
	if constraints.Check(versionPassed) {
		if instance.Spec.General.HttpPort > 0 {
			httpPort = instance.Spec.General.HttpPort
		} else {
			httpPort = 9200
		}
		securityconfigPath = "/usr/share/opensearch/config/opensearch-security"
	} else {
		httpPort = 9300
		securityconfigPath = "/usr/share/opensearch/plugins/opensearch-security/securityconfig"
	}
	return httpPort, securityconfigPath
}
