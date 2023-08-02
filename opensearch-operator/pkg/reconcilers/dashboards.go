package reconcilers

import (
	"context"
	"fmt"

	"github.com/cisco-open/operator-tools/pkg/reconciler"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/builders"
	"opensearch.opster.io/pkg/helpers"
	"opensearch.opster.io/pkg/reconcilers/util"
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
	pki               tls.PKI
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
		pki:               tls.NewPKI(),
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

	// add any aditional dashboard config to the reconciler context
	for key, value := range r.instance.Spec.Dashboards.AdditionalConfig {
		r.reconcilerContext.AddDashboardsConfig(key, value)
	}

	// Generate additional volumes
	addVolumes, addVolumeMounts, _, err := util.CreateAdditionalVolumes(
		r.ctx,
		r.Client,
		r.instance.Namespace,
		r.instance.Spec.Dashboards.AdditionalVolumes,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	volumes = append(volumes, addVolumes...)
	volumeMounts = append(volumeMounts, addVolumeMounts...)

	cm := builders.NewDashboardsConfigMapForCR(r.instance, fmt.Sprintf("%s-dashboards-config", r.instance.Name), r.reconcilerContext.DashboardsConfig)
	result.CombineErr(ctrl.SetControllerReference(r.instance, cm, r.Client.Scheme()))
	result.Combine(r.ReconcileResource(cm, reconciler.StatePresent))

	annotations := make(map[string]string)

	if cmData, ok := cm.Data[helpers.DashboardConfigName]; ok {
		sha1sum, err := util.GetSha1Sum([]byte(cmData))
		if err != nil {
			return ctrl.Result{}, err
		}

		annotations[helpers.DashboardChecksumName] = sha1sum
	}

	deployment := builders.NewDashboardsDeploymentForCR(r.instance, volumes, volumeMounts, annotations)
	result.CombineErr(ctrl.SetControllerReference(r.instance, deployment, r.Client.Scheme()))
	result.Combine(r.ReconcileResource(deployment, reconciler.StatePresent))

	svc := builders.NewDashboardsSvcForCr(r.instance)
	result.CombineErr(ctrl.SetControllerReference(r.instance, svc, r.Client.Scheme()))
	result.Combine(r.ReconcileResource(svc, reconciler.StatePresent))

	return result.Result, result.Err
}

func (r *DashboardsReconciler) handleTls() ([]corev1.Volume, []corev1.VolumeMount, error) {
	if r.instance.Spec.Dashboards.Tls == nil || !r.instance.Spec.Dashboards.Tls.Enable {
		return nil, nil, nil
	}
	clusterName := r.instance.Name
	namespace := r.instance.Namespace
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	tlsSecretName := clusterName + "-dashboards-cert"
	tlsConfig := r.instance.Spec.Dashboards.Tls
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	if tlsConfig.Generate {
		r.logger.Info("Generating certificates")
		r.recorder.AnnotatedEventf(r.instance, annotations, "Info", "Security", "Starting to generating certificates for Dashboard Cluster")
		// Take CA from TLS reconciler or generate new one
		var ca tls.Cert
		var err error
		if tlsConfig.TlsCertificateConfig.CaSecret.Name != "" {
			ca, err = r.providedCaCert(tlsConfig.TlsCertificateConfig.CaSecret.Name, namespace)
		} else {
			ca, err = util.ReadOrGenerateCaCert(r.pki, r.Client, r.ctx, r.instance)
		}
		if err != nil {
			return volumes, volumeMounts, err
		}

		// Generate cert and create secret
		tlsSecret := corev1.Secret{}
		if err := r.Get(r.ctx, client.ObjectKey{Name: tlsSecretName, Namespace: namespace}, &tlsSecret); err != nil {
			// Generate tls cert and put it into secret
			dnsNames := []string{
				fmt.Sprintf("%s-dashboards", clusterName),
				fmt.Sprintf("%s-dashboards.%s", clusterName, namespace),
				fmt.Sprintf("%s-dashboards.%s.svc", clusterName, namespace),
				fmt.Sprintf("%s-dashboards.%s.svc.%s", clusterName, namespace, helpers.ClusterDnsBase()),
			}
			nodeCert, err := ca.CreateAndSignCertificate(clusterName+"-dashboards", clusterName, dnsNames)
			if err != nil {
				r.logger.Error(err, "Failed to create tls certificate")
				r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Security", "Failed to store tls certificate for Dashboard Cluster")
				return volumes, volumeMounts, err
			}
			tlsSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: namespace}, Data: nodeCert.SecretData(ca)}
			if err := ctrl.SetControllerReference(r.instance, &tlsSecret, r.Client.Scheme()); err != nil {
				return nil, nil, err
			}
			if err := r.Create(r.ctx, &tlsSecret); err != nil {
				r.logger.Error(err, "Failed to store tls certificate in secret")
				r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Security", "Failed to store tls certificate for Dashboard Cluster")
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
		r.recorder.AnnotatedEventf(r.instance, annotations, "Info", "Security", "Notice - using externally provided certificates for Dashboard Cluster")
		volume := corev1.Volume{Name: "tls-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: tlsConfig.TlsCertificateConfig.Secret.Name}}}
		volumes = append(volumes, volume)
		mount := corev1.VolumeMount{Name: "tls-cert", MountPath: "/usr/share/opensearch-dashboards/certs"}
		volumeMounts = append(volumeMounts, mount)
	}
	// Update dashboards config
	r.reconcilerContext.AddDashboardsConfig("server.ssl.enabled", "true")
	r.reconcilerContext.AddDashboardsConfig("server.ssl.key", "/usr/share/opensearch-dashboards/certs/tls.key")
	r.reconcilerContext.AddDashboardsConfig("server.ssl.certificate", "/usr/share/opensearch-dashboards/certs/tls.crt")
	return volumes, volumeMounts, nil
}

func (r *DashboardsReconciler) providedCaCert(secretName string, namespace string) (tls.Cert, error) {
	var ca tls.Cert
	caSecret := corev1.Secret{}
	if err := r.Get(r.ctx, client.ObjectKey{Name: secretName, Namespace: namespace}, &caSecret); err != nil {
		return ca, err
	}
	ca = r.pki.CAFromSecret(caSecret.Data)
	return ca, nil
}

func (r *DashboardsReconciler) DeleteResources() (ctrl.Result, error) {
	result := reconciler.CombinedResult{}
	return result.Result, result.Err
}
