package controllers

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	sts "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/builders"
	tls "opensearch.opster.io/pkg/tls"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DashboardReconciler struct {
	client.Client
	Recorder record.EventRecorder
	logr.Logger
	Instance     *opsterv1.OpenSearchCluster
	volumes      []corev1.Volume
	volumeMounts []corev1.VolumeMount
}

func (r *DashboardReconciler) Reconcile(controllerContext *ControllerContext) (*opsterv1.ComponentStatus, error) {
	r.Logger.Info("Starting dashboards reconcile")
	namespace := r.Instance.Spec.General.ClusterName

	if err := r.handleTls(controllerContext); err != nil {
		return nil, err
	}

	/// ------ create opensearch dashboard cm ------- ///
	kibanaCm := corev1.ConfigMap{}
	cmName := fmt.Sprintf("%s-dashboards-config", r.Instance.Spec.General.ClusterName)
	if err := r.Get(context.TODO(), client.ObjectKey{Name: cmName, Namespace: namespace}, &kibanaCm); err != nil {
		dashboards_cm := builders.NewDashboardsConfigMapForCR(r.Instance, cmName, controllerContext.DashboardsConfig)

		err = r.Create(context.TODO(), dashboards_cm)
		if err != nil {
			if !k8serrors.IsAlreadyExists(err) {
				r.Logger.Error(err, "Cannot create Opensearch-Dashboard Configmap "+dashboards_cm.Name)
				r.Recorder.Event(r.Instance, "Warning", "Cannot create OpenSearch-Dashboard configmap ", "Fix the problem you have on main Opensearch-Dashboard ConfigMap")
				return nil, err
			}
		}
		r.Logger.Info("Opensearch-Dashboard Cm Created successfully", "name", dashboards_cm.Name)

	}

	kibanaDeploy := sts.Deployment{}
	deployName := r.Instance.Spec.General.ClusterName + "-dashboards"

	if err := r.Get(context.TODO(), client.ObjectKey{Name: deployName, Namespace: namespace}, &kibanaDeploy); err != nil {
		/// ------- create Opensearch-Dashboard deployment ------- ///
		dashboards_deployment := builders.NewDashboardsDeploymentForCR(r.Instance, r.volumes, r.volumeMounts)

		err = r.Create(context.TODO(), dashboards_deployment)
		if err != nil {
			if !k8serrors.IsAlreadyExists(err) {
				r.Logger.Error(err, "Cannot create Opensearch-Dashboard Deployment "+dashboards_deployment.Name)
				r.Recorder.Event(r.Instance, "Warning", "Cannot create OpenSearch-Dashboard deployment ", "Fix the problem you have on main Opensearch-Dashboard Deployment")
				return nil, err
			}
		}
		r.Logger.Info("Opensearch-Dashboard Deployment Created successfully - ", "name : ", dashboards_deployment.Name)
	}

	kibanaService := corev1.Service{}
	serviceName := r.Instance.Spec.General.ServiceName + "-dashboards"

	if err := r.Get(context.TODO(), client.ObjectKey{Name: serviceName, Namespace: namespace}, &kibanaService); err != nil {
		/// -------- create Opensearch-Dashboard service ------- ///
		dashboards_svc := builders.NewDashboardsSvcForCr(r.Instance)
		err = r.Create(context.TODO(), dashboards_svc)
		if err != nil {
			if !k8serrors.IsAlreadyExists(err) {
				r.Logger.Error(err, "Cannot create Opensearch-Dashboard service "+dashboards_svc.Name)
				r.Recorder.Event(r.Instance, "Warning", "Cannot create OpenSearch-Dashboard service ", "Fix the problem you have on main Opensearch-Dashboard Service")
				return nil, err
			}
		}
		r.Logger.Info("Opensearch-Dashboard service Created successfully", "name", dashboards_svc.Name)
	}

	return nil, nil
}

func (r *DashboardReconciler) handleTls(controllerContext *ControllerContext) error {
	if r.Instance.Spec.Dashboards.Tls == nil || !r.Instance.Spec.Dashboards.Tls.Enable {
		return nil
	}
	clusterName := r.Instance.Spec.General.ClusterName
	namespace := clusterName
	caSecretName := clusterName + "-ca"
	tlsSecretName := clusterName + "-dashboards-cert"
	tlsConfig := r.Instance.Spec.Dashboards.Tls
	if tlsConfig.Generate {
		r.Logger.Info("Generating certificates")
		// Take CA from TLS reconciler or generate new one
		ca, err := r.caCert(caSecretName, namespace, clusterName)
		if err != nil {
			return err
		}
		// Generate cert and create secret
		tlsSecret := corev1.Secret{}
		if err := r.Get(context.TODO(), client.ObjectKey{Name: tlsSecretName, Namespace: namespace}, &tlsSecret); err != nil {
			// Generate tls cert and put it into secret
			dnsNames := []string{
				fmt.Sprintf("%s-dashboards", clusterName),
				fmt.Sprintf("%s-dashboards.%s", clusterName, namespace),
				fmt.Sprintf("%s-dashboards.%s.svc", clusterName, namespace),
				fmt.Sprintf("%s-dashboards.%s.svc.cluster.local", clusterName, namespace),
			}
			nodeCert, err := ca.CreateAndSignCertificate(clusterName+"-dashboards", clusterName, dnsNames)
			if err != nil {
				r.Logger.Error(err, "Failed to create tls certificate")
				return err
			}
			tlsSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: namespace}, Data: nodeCert.SecretData(&ca)}
			if err := r.Create(context.TODO(), &tlsSecret); err != nil {
				r.Logger.Error(err, "Failed to store tls certificate in secret")
				return err
			}
		}
		// Mount secret
		volume := corev1.Volume{Name: "tls-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: tlsSecretName}}}
		r.volumes = append(r.volumes, volume)
		mount := corev1.VolumeMount{Name: "tls-cert", MountPath: "/usr/share/opensearch-dashboards/certs"}
		r.volumeMounts = append(r.volumeMounts, mount)
	} else {
		r.Logger.Info("Using externally provided certificates")
		if tlsConfig.Secret != "" {
			volume := corev1.Volume{Name: "tls-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: tlsConfig.Secret}}}
			r.volumes = append(r.volumes, volume)
			mount := corev1.VolumeMount{Name: "tls-cert", MountPath: "/usr/share/opensearch-dashboards/certs"}
			r.volumeMounts = append(r.volumeMounts, mount)
		} else {
			if tlsConfig.CertSecret == nil || tlsConfig.KeySecret == nil {
				err := errors.New("generate=false but certSecret or keySecret not provided")
				r.Logger.Error(err, "Secret not provided")
				return err
			}
			secretKey := "tls.key"
			if tlsConfig.KeySecret.Key != nil {
				secretKey = *tlsConfig.KeySecret.Key
			}
			volume := corev1.Volume{Name: "tls-key", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: tlsConfig.KeySecret.SecretName}}}
			r.volumes = append(r.volumes, volume)
			mount := corev1.VolumeMount{Name: "tls-key", MountPath: "/usr/share/opensearch-dashboards/certs/tls.key", SubPath: secretKey}
			r.volumeMounts = append(r.volumeMounts, mount)
			secretKey = "tls.crt"
			if tlsConfig.CertSecret.Key != nil {
				secretKey = *tlsConfig.CertSecret.Key
			}
			volume = corev1.Volume{Name: "tls-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: tlsConfig.CertSecret.SecretName}}}
			r.volumes = append(r.volumes, volume)
			mount = corev1.VolumeMount{Name: "tls-cert", MountPath: "/usr/share/opensearch-dashboards/certs/tls.crt", SubPath: secretKey}
			r.volumeMounts = append(r.volumeMounts, mount)
		}
	}
	// Update dashboards config
	controllerContext.AddDashboardsConfig("server.ssl.enabled", "true")
	controllerContext.AddDashboardsConfig("server.ssl.key", "/usr/share/opensearch-dashboards/certs/tls.key")
	controllerContext.AddDashboardsConfig("server.ssl.certificate", "/usr/share/opensearch-dashboards/certs/tls.crt")
	return nil
}

// TODO: Move to helpers and merge with method from tlscontroller
func (r *DashboardReconciler) caCert(secretName string, namespace string, clusterName string) (tls.Cert, error) {
	caSecret := corev1.Secret{}
	var ca tls.Cert
	if err := r.Get(context.TODO(), client.ObjectKey{Name: secretName, Namespace: namespace}, &caSecret); err != nil {
		// Generate CA cert and put it into secret
		ca, err = tls.GenerateCA(clusterName)
		if err != nil {
			r.Logger.Error(err, "Failed to create CA")
			return ca, err
		}
		caSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace}, Data: ca.SecretDataCA()}
		if err := r.Create(context.TODO(), &caSecret); err != nil {
			r.Logger.Error(err, "Failed to store CA in secret")
			return ca, err
		}
	} else {
		ca = tls.CAFromSecret(caSecret.Data)
	}
	return ca, nil
}
