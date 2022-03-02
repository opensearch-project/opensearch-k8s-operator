package reconcilers

import (
	"context"
	"errors"
	"fmt"

	"github.com/banzaicloud/operator-tools/pkg/reconciler"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/builders"
	"opensearch.opster.io/pkg/tls"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type DashboardsReconciler struct {
	reconciler.ResourceReconciler
	client.Client
	ctx               context.Context
	recorder          record.EventRecorder
	reconcilerContext *ReconcilerContext
	instance          *opsterv1.OpenSearchCluster
	logger            logr.Logger
}

func NewDashboardsReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	reconcilerContext *ReconcilerContext,
	instance *opsterv1.OpenSearchCluster,
	opts ...reconciler.ResourceReconcilerOption,
) *DashboardsReconciler {
	return &DashboardsReconciler{
		Client: client,
		ResourceReconciler: reconciler.NewReconcilerWith(client,
			append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "dashboards")))...),
		ctx:               ctx,
		reconcilerContext: reconcilerContext,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx),
	}
}

func (r *DashboardsReconciler) Reconcile() (ctrl.Result, error) {
	if !r.instance.Spec.Dashboards.Enable {
		return ctrl.Result{}, nil
	}
	result := reconciler.CombinedResult{}

	volumes, volumeMounts, err := r.handleTls()
	if err != nil {
		return ctrl.Result{}, err
	}

	cm := builders.NewDashboardsConfigMapForCR(r.instance, fmt.Sprintf("%s-dashboards-config", r.instance.Spec.General.ClusterName), r.reconcilerContext.DashboardsConfig)
	result.Combine(r.ReconcileResource(cm, reconciler.StatePresent))

	deployment := builders.NewDashboardsDeploymentForCR(r.instance, volumes, volumeMounts)
	result.Combine(r.ReconcileResource(deployment, reconciler.StatePresent))

	svc := builders.NewDashboardsSvcForCr(r.instance)
	result.Combine(r.ReconcileResource(svc, reconciler.StatePresent))

	return result.Result, result.Err
}

func (r *DashboardsReconciler) handleTls() ([]corev1.Volume, []corev1.VolumeMount, error) {
	if r.instance.Spec.Dashboards.Tls == nil || !r.instance.Spec.Dashboards.Tls.Enable {
		return nil, nil, nil
	}
	clusterName := r.instance.Spec.General.ClusterName
	namespace := clusterName
	caSecretName := clusterName + "-ca"
	tlsSecretName := clusterName + "-dashboards-cert"
	tlsConfig := r.instance.Spec.Dashboards.Tls
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	if tlsConfig.Generate {
		r.logger.Info("Generating certificates")
		// Take CA from TLS reconciler or generate new one
		ca, err := r.caCert(caSecretName, namespace, clusterName)
		if err != nil {
			return volumes, volumeMounts, err
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
				r.logger.Error(err, "Failed to create tls certificate")
				return volumes, volumeMounts, err
			}
			tlsSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: namespace}, Data: nodeCert.SecretData(&ca)}
			if err := r.Create(context.TODO(), &tlsSecret); err != nil {
				r.logger.Error(err, "Failed to store tls certificate in secret")
				return volumes, volumeMounts, err
			}
		}
		// Mount secret
		volume := corev1.Volume{Name: "tls-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: tlsSecretName}}}
		volumes = append(volumes, volume)
		mount := corev1.VolumeMount{Name: "tls-cert", MountPath: "/usr/share/opensearch-dashboards/certs"}
		volumeMounts = append(volumeMounts, mount)
	} else {
		r.logger.Info("Using externally provided certificates")
		if tlsConfig.Secret != "" {
			volume := corev1.Volume{Name: "tls-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: tlsConfig.Secret}}}
			volumes = append(volumes, volume)
			mount := corev1.VolumeMount{Name: "tls-cert", MountPath: "/usr/share/opensearch-dashboards/certs"}
			volumeMounts = append(volumeMounts, mount)
		} else {
			if tlsConfig.CertSecret == nil || tlsConfig.KeySecret == nil {
				err := errors.New("generate=false but certSecret or keySecret not provided")
				r.logger.Error(err, "Secret not provided")
				return volumes, volumeMounts, err
			}
			secretKey := "tls.key"
			if tlsConfig.KeySecret.Key != nil {
				secretKey = *tlsConfig.KeySecret.Key
			}
			volume := corev1.Volume{Name: "tls-key", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: tlsConfig.KeySecret.SecretName}}}
			volumes = append(volumes, volume)
			mount := corev1.VolumeMount{Name: "tls-key", MountPath: "/usr/share/opensearch-dashboards/certs/tls.key", SubPath: secretKey}
			volumeMounts = append(volumeMounts, mount)
			secretKey = "tls.crt"
			if tlsConfig.CertSecret.Key != nil {
				secretKey = *tlsConfig.CertSecret.Key
			}
			volume = corev1.Volume{Name: "tls-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: tlsConfig.CertSecret.SecretName}}}
			volumes = append(volumes, volume)
			mount = corev1.VolumeMount{Name: "tls-cert", MountPath: "/usr/share/opensearch-dashboards/certs/tls.crt", SubPath: secretKey}
			volumeMounts = append(volumeMounts, mount)
		}
	}
	// Update dashboards config
	r.reconcilerContext.AddDashboardsConfig("server.ssl.enabled", "true")
	r.reconcilerContext.AddDashboardsConfig("server.ssl.key", "/usr/share/opensearch-dashboards/certs/tls.key")
	r.reconcilerContext.AddDashboardsConfig("server.ssl.certificate", "/usr/share/opensearch-dashboards/certs/tls.crt")
	return volumes, volumeMounts, nil
}

// TODO: Move to helpers and merge with method from tlscontroller
func (r *DashboardsReconciler) caCert(secretName string, namespace string, clusterName string) (tls.Cert, error) {
	caSecret := corev1.Secret{}
	var ca tls.Cert
	if err := r.Get(context.TODO(), client.ObjectKey{Name: secretName, Namespace: namespace}, &caSecret); err != nil {
		// Generate CA cert and put it into secret
		ca, err = tls.GenerateCA(clusterName)
		if err != nil {
			r.logger.Error(err, "Failed to create CA")
			return ca, err
		}
		caSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace}, Data: ca.SecretDataCA()}
		if err := r.Create(context.TODO(), &caSecret); err != nil {
			r.logger.Error(err, "Failed to store CA in secret")
			return ca, err
		}
	} else {
		ca = tls.CAFromSecret(caSecret.Data)
	}
	return ca, nil
}
