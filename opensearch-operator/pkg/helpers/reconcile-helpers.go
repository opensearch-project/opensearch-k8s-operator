package helpers

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-openapi/pkg/validation/errors"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/tls"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func CreateInitMasters(cr *opsterv1.OpenSearchCluster) string {
	var masters []string
	for _, nodePool := range cr.Spec.NodePools {
		if ContainsString(nodePool.Roles, "master") {
			for i := 0; int32(i) < nodePool.Replicas; i++ {
				masters = append(masters, fmt.Sprintf("%s-%s-%d", cr.Name, nodePool.Component, i))
			}
		}
	}
	return strings.Join(masters, ",")
}

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
