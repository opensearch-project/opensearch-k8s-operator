package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
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

type TLSReconciler struct {
	reconciler.ResourceReconciler
	client.Client
	ctx               context.Context
	reconcilerContext *ReconcilerContext
	instance          *opsterv1.OpenSearchCluster
	logger            logr.Logger
	pki               tls.PKI
	recorder          record.EventRecorder
}

func NewTLSReconciler(
	client client.Client,
	ctx context.Context,
	reconcilerContext *ReconcilerContext,
	instance *opsterv1.OpenSearchCluster,
	opts ...reconciler.ResourceReconcilerOption,
) *TLSReconciler {
	return &TLSReconciler{
		Client: client,
		ResourceReconciler: reconciler.NewReconcilerWith(client,
			append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "tls")))...),
		ctx:               ctx,
		reconcilerContext: reconcilerContext,
		instance:          instance,
		logger:            log.FromContext(ctx),
		pki:               tls.NewPKI(),
	}
}

const (
	CaCertKey = "ca.crt"
)

func (r *TLSReconciler) Reconcile() (ctrl.Result, error) {

	if r.instance.Spec.Security == nil || r.instance.Spec.Security.Tls == nil {
		r.logger.Info("No security specified. Not doing anything")
		return ctrl.Result{}, nil
	}

	tlsConfig := r.instance.Spec.Security.Tls

	if tlsConfig.Transport != nil {
		if err := r.handleTransport(); err != nil {
			return ctrl.Result{}, err
		}
	}
	if tlsConfig.Http != nil {
		if err := r.handleHttp(); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *TLSReconciler) handleTransport() error {
	config := r.instance.Spec.Security.Tls.Transport
	if config.Generate {
		if config.PerNode {
			if err := r.handleTransportGeneratePerNode(); err != nil {
				return err
			}
		} else {
			if err := r.handleTransportGenerateGlobal(); err != nil {
				return err
			}
		}
	} else {
		if err := r.handleTransportExistingCerts(); err != nil {
			return err
		}
	}
	err := r.handleAdminCertificate()
	return err
}

func (r *TLSReconciler) handleAdminCertificate() error {
	tlsConfig := r.instance.Spec.Security.Tls.Transport
	clusterName := r.instance.Name

	if tlsConfig.Generate {
		ca, err := r.getCACert()
		if err != nil {
			return err
		}
		err = r.createAdminSecret(ca)
		if err != nil {
			return err
		}
		r.reconcilerContext.AddConfig("plugins.security.authcz.admin_dn", fmt.Sprintf("[\"CN=admin,OU=%s\"]", clusterName))
		return nil
	}

	adminDn := strings.Join(tlsConfig.AdminDn, "\",\"")
	r.reconcilerContext.AddConfig("plugins.security.authcz.admin_dn", fmt.Sprintf("[\"%s\"]", adminDn))
	return nil
}

func (r *TLSReconciler) securityChangeVersion() bool {
	newVersionConstraint, err := semver.NewConstraint(">=2.0.0")
	if err != nil {
		panic(err)
	}

	version, err := semver.NewVersion(r.instance.Spec.General.Version)
	if err != nil {
		r.logger.Error(err, "unable to parse version, assuming >= 2.0.0")
		return true
	}
	return newVersionConstraint.Check(version)
}

func (r *TLSReconciler) adminCAProvided() bool {
	if r.securityChangeVersion() {
		return r.instance.Spec.Security.Tls.Http.TlsCertificateConfig.CaSecret.Name != ""
	}
	return r.instance.Spec.Security.Tls.Transport.TlsCertificateConfig.CaSecret.Name != ""
}

func (r *TLSReconciler) providedCAForAdminCert() (tls.Cert, error) {
	if r.securityChangeVersion() {
		return r.providedCaCert(
			r.instance.Spec.Security.Tls.Http.TlsCertificateConfig.CaSecret.Name,
			r.instance.Namespace,
		)
	}
	return r.providedCaCert(
		r.instance.Spec.Security.Tls.Transport.TlsCertificateConfig.CaSecret.Name,
		r.instance.Namespace,
	)
}

func (r *TLSReconciler) getCACert() (tls.Cert, error) {
	if r.adminCAProvided() {
		return r.providedCAForAdminCert()
	}
	return util.ReadOrGenerateCaCert(r.pki, r.Client, r.ctx, r.instance)
}

func (r *TLSReconciler) createAdminSecret(ca tls.Cert) error {
	adminCert, err := ca.CreateAndSignCertificate("admin", r.instance.Name, nil)
	if err != nil {
		r.logger.Error(err, "Failed to create admin certificate", "interface", "transport")
		r.recorder.AnnotatedEventf(
			r.instance,
			map[string]string{"cluster-name": r.instance.GetName()},
			"Warning",
			"Security",
			"Failed to create admin certificate",
		)
		return err
	}
	adminSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.adminSecretName(),
			Namespace: r.instance.Namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: adminCert.SecretData(ca),
	}
	if err := ctrl.SetControllerReference(r.instance, adminSecret, r.Client.Scheme()); err != nil {
		return err
	}
	return client.IgnoreAlreadyExists(r.Create(r.ctx, adminSecret))
}

func (r *TLSReconciler) adminSecretName() string {
	return r.instance.Name + "-admin-cert"
}

func (r *TLSReconciler) handleTransportGenerateGlobal() error {
	namespace := r.instance.Namespace
	clusterName := r.instance.Name
	nodeSecretName := clusterName + "-transport-cert"

	r.logger.Info("Generating certificates", "interface", "transport")
	//r.recorder.Event(r.instance, "Normal", "Security", "Starting to generating certificates")

	var ca tls.Cert
	var err error
	if r.instance.Spec.Security.Tls.Transport.TlsCertificateConfig.CaSecret.Name != "" {
		ca, err = r.providedCaCert(r.instance.Spec.Security.Tls.Transport.TlsCertificateConfig.CaSecret.Name, namespace)
	} else {
		ca, err = util.ReadOrGenerateCaCert(r.pki, r.Client, r.ctx, r.instance)
	}
	if err != nil {
		return err
	}

	// Generate node cert, sign it and put it into secret
	nodeSecret := corev1.Secret{}
	if err := r.Get(r.ctx, client.ObjectKey{Name: nodeSecretName, Namespace: namespace}, &nodeSecret); err != nil {
		// Generate node cert and put it into secret
		dnsNames := []string{
			clusterName,
			fmt.Sprintf("%s.%s", clusterName, namespace),
			fmt.Sprintf("%s.%s.svc", clusterName, namespace),
			fmt.Sprintf("%s.%s.svc.%s", clusterName, namespace, helpers.ClusterDnsBase()),
		}
		nodeCert, err := ca.CreateAndSignCertificate(clusterName, clusterName, dnsNames)
		if err != nil {
			r.logger.Error(err, "Failed to create node certificate", "interface", "transport")
			return err
		}
		nodeSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: nodeSecretName, Namespace: namespace}, Type: corev1.SecretTypeTLS, Data: nodeCert.SecretData(ca)}
		if err := ctrl.SetControllerReference(r.instance, &nodeSecret, r.Client.Scheme()); err != nil {
			return err
		}
		if err := r.Create(r.ctx, &nodeSecret); err != nil {
			r.logger.Error(err, "Failed to store node certificate in secret", "interface", "transport")
			return err
		}
	}
	// Tell cluster controller to mount secrets
	volume := corev1.Volume{Name: "transport-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: nodeSecretName}}}
	r.reconcilerContext.Volumes = append(r.reconcilerContext.Volumes, volume)
	mount := corev1.VolumeMount{Name: "transport-cert", MountPath: "/usr/share/opensearch/config/tls-transport"}
	r.reconcilerContext.VolumeMounts = append(r.reconcilerContext.VolumeMounts, mount)
	// Extend opensearch.yml
	r.reconcilerContext.AddConfig("plugins.security.nodes_dn", fmt.Sprintf("[\"CN=%s,OU=%s\"]", clusterName, clusterName))
	r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemcert_filepath", fmt.Sprintf("tls-transport/%s", corev1.TLSCertKey))
	r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemkey_filepath", fmt.Sprintf("tls-transport/%s", corev1.TLSPrivateKeyKey))
	r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemtrustedcas_filepath", fmt.Sprintf("tls-transport/%s", CaCertKey))
	r.reconcilerContext.AddConfig("plugins.security.ssl.transport.enforce_hostname_verification", "false")
	return nil
}

func (r *TLSReconciler) handleTransportGeneratePerNode() error {
	r.logger.Info("Generating certificates", "interface", "transport")
	//r.recorder.Event(r.instance, "Normal", "Security", "Start to generating certificates")

	namespace := r.instance.Namespace
	clusterName := r.instance.Name
	nodeSecretName := clusterName + "-transport-cert"

	var ca tls.Cert
	var err error
	if r.instance.Spec.Security.Tls.Transport.TlsCertificateConfig.CaSecret.Name != "" {
		ca, err = r.providedCaCert(r.instance.Spec.Security.Tls.Transport.TlsCertificateConfig.CaSecret.Name, namespace)
	} else {
		ca, err = util.ReadOrGenerateCaCert(r.pki, r.Client, r.ctx, r.instance)
	}
	if err != nil {
		return err
	}

	nodeSecret := corev1.Secret{}
	exists := true
	if err := r.Get(r.ctx, client.ObjectKey{Name: nodeSecretName, Namespace: namespace}, &nodeSecret); err != nil {
		nodeSecret.Data = make(map[string][]byte)
		nodeSecret.ObjectMeta = metav1.ObjectMeta{Name: nodeSecretName, Namespace: namespace}
		exists = false
	}
	nodeSecret.Data[CaCertKey] = ca.CertData()

	// Generate bootstrap pod cert
	bootstrapPodName := builders.BootstrapPodName(r.instance)
	_, bootstrapCertExists := nodeSecret.Data[fmt.Sprintf("%s.crt", bootstrapPodName)]
	_, bootstrapKeyExists := nodeSecret.Data[fmt.Sprintf("%s.key", bootstrapPodName)]

	if !r.instance.Status.Initialized && !(bootstrapCertExists && bootstrapKeyExists) {
		dnsNames := []string{
			bootstrapPodName,
			clusterName,
			builders.DiscoveryServiceName(r.instance),
			fmt.Sprintf("%s.%s", bootstrapPodName, clusterName),
			fmt.Sprintf("%s.%s", clusterName, namespace),
			fmt.Sprintf("%s.%s.%s", bootstrapPodName, clusterName, namespace),
			fmt.Sprintf("%s.%s.svc", clusterName, namespace),
			fmt.Sprintf("%s.%s.%s.svc", bootstrapPodName, clusterName, namespace),
			fmt.Sprintf("%s.%s.svc.%s", clusterName, namespace, helpers.ClusterDnsBase()),
			fmt.Sprintf("%s.%s.%s.svc.%s", bootstrapPodName, clusterName, namespace, helpers.ClusterDnsBase()),
		}
		nodeCert, err := ca.CreateAndSignCertificate(bootstrapPodName, clusterName, dnsNames)
		if err != nil {
			r.logger.Error(err, "Failed to create node certificate", "interface", "transport", "node", bootstrapPodName)
			//	r.recorder.Event(r.instance, "Normal", "Security", "Created transport certificates")
			return err
		}
		//	r.recorder.Event(r.instance, "Normal", "Security", "Created transport certificates")
		nodeSecret.Data[fmt.Sprintf("%s.crt", bootstrapPodName)] = nodeCert.CertData()
		nodeSecret.Data[fmt.Sprintf("%s.key", bootstrapPodName)] = nodeCert.KeyData()
	}

	// Generate node cert and put it into secret
	for _, nodePool := range r.instance.Spec.NodePools {
		for i := 0; i < int(nodePool.Replicas); i++ {
			podName := fmt.Sprintf("%s-%s-%d", clusterName, nodePool.Component, i)
			certName := fmt.Sprintf("%s.crt", podName)
			keyName := fmt.Sprintf("%s.key", podName)
			_, certExists := nodeSecret.Data[certName]
			_, keyExists := nodeSecret.Data[keyName]
			if certExists && keyExists {
				continue
			}
			dnsNames := []string{
				podName,
				clusterName,
				builders.DiscoveryServiceName(r.instance),
				fmt.Sprintf("%s.%s", podName, clusterName),
				fmt.Sprintf("%s.%s", clusterName, namespace),
				fmt.Sprintf("%s.%s.%s", podName, clusterName, namespace),
				fmt.Sprintf("%s.%s.svc", clusterName, namespace),
				fmt.Sprintf("%s.%s.%s.svc", podName, clusterName, namespace),
				fmt.Sprintf("%s.%s.svc.%s", clusterName, namespace, helpers.ClusterDnsBase()),
				fmt.Sprintf("%s.%s.%s.svc.%s", podName, clusterName, namespace, helpers.ClusterDnsBase()),
			}
			nodeCert, err := ca.CreateAndSignCertificate(podName, clusterName, dnsNames)
			if err != nil {
				r.logger.Error(err, "Failed to create node certificate", "interface", "transport", "node", podName)
				return err
			}
			nodeSecret.Data[certName] = nodeCert.CertData()
			nodeSecret.Data[keyName] = nodeCert.KeyData()
		}
	}
	if exists {
		if err := r.Update(r.ctx, &nodeSecret); err != nil {
			r.logger.Error(err, "Failed to store node certificate in secret", "interface", "transport")
			return err
		}
	} else {
		if err := ctrl.SetControllerReference(r.instance, &nodeSecret, r.Client.Scheme()); err != nil {
			return err
		}
		if err := r.Create(r.ctx, &nodeSecret); err != nil {
			r.logger.Error(err, "Failed to store node certificate in secret", "interface", "transport")
			return err
		}
	}
	// Tell cluster controller to mount secrets
	volume := corev1.Volume{Name: "transport-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: nodeSecretName}}}
	r.reconcilerContext.Volumes = append(r.reconcilerContext.Volumes, volume)
	mount := corev1.VolumeMount{Name: "transport-cert", MountPath: "/usr/share/opensearch/config/tls-transport"}
	r.reconcilerContext.VolumeMounts = append(r.reconcilerContext.VolumeMounts, mount)

	// Extend opensearch.yml
	r.reconcilerContext.AddConfig("plugins.security.nodes_dn", fmt.Sprintf("[\"CN=%s-*,OU=%s\"]", clusterName, clusterName))
	r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemcert_filepath", "tls-transport/${HOSTNAME}.crt")
	r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemkey_filepath", "tls-transport/${HOSTNAME}.key")
	r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemtrustedcas_filepath", fmt.Sprintf("tls-transport/%s", CaCertKey))
	r.reconcilerContext.AddConfig("plugins.security.ssl.transport.enforce_hostname_verification", "true")
	return nil
}

func (r *TLSReconciler) handleTransportExistingCerts() error {
	tlsConfig := r.instance.Spec.Security.Tls.Transport
	if tlsConfig.PerNode {
		if tlsConfig.TlsCertificateConfig.Secret.Name == "" {
			err := errors.New("perNode=true but secret not set")
			r.logger.Error(err, "Secret not provided")
			//		r.recorder.Event(r.instance, "Warning", "Security", "Notice - perNode=true but secret not set but Secret not provided")
			return err
		}
		mountFolder("transport", "certs", tlsConfig.TlsCertificateConfig.Secret.Name, r.reconcilerContext)
		// Extend opensearch.yml
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemcert_filepath", "tls-transport/${HOSTNAME}.crt")
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemkey_filepath", "tls-transport/${HOSTNAME}.key")
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.enforce_hostname_verification", "true")
	} else {
		if tlsConfig.TlsCertificateConfig.Secret.Name == "" {
			err := errors.New("missing secret in spec")
			r.logger.Error(err, "Not all secrets for transport provided")
			//		r.recorder.Event(r.instance, "Warning", "Security", "Notice - Not all secrets for transport provided")
			return err
		}
		if tlsConfig.TlsCertificateConfig.CaSecret.Name == "" {
			mountFolder("transport", "certs", tlsConfig.TlsCertificateConfig.Secret.Name, r.reconcilerContext)
		} else {
			mount("transport", "ca", CaCertKey, tlsConfig.TlsCertificateConfig.CaSecret.Name, r.reconcilerContext)
			mount("transport", "key", corev1.TLSPrivateKeyKey, tlsConfig.TlsCertificateConfig.Secret.Name, r.reconcilerContext)
			mount("transport", "cert", corev1.TLSCertKey, tlsConfig.TlsCertificateConfig.Secret.Name, r.reconcilerContext)
		}
		// Extend opensearch.yml
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemcert_filepath", fmt.Sprintf("tls-transport/%s", corev1.TLSCertKey))
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemkey_filepath", fmt.Sprintf("tls-transport/%s", corev1.TLSPrivateKeyKey))
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.enforce_hostname_verification", "false")
	}
	r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemtrustedcas_filepath", fmt.Sprintf("tls-transport/%s", CaCertKey))
	dnList := strings.Join(tlsConfig.NodesDn, "\",\"")
	r.reconcilerContext.AddConfig("plugins.security.nodes_dn", fmt.Sprintf("[\"%s\"]", dnList))
	return nil
}

func (r *TLSReconciler) handleHttp() error {
	tlsConfig := r.instance.Spec.Security.Tls.Http
	namespace := r.instance.Namespace
	clusterName := r.instance.Name
	nodeSecretName := clusterName + "-http-cert"

	if tlsConfig.Generate {
		r.logger.Info("Generating certificates", "interface", "http")

		var ca tls.Cert
		var err error
		if tlsConfig.TlsCertificateConfig.CaSecret.Name != "" {
			ca, err = r.providedCaCert(tlsConfig.TlsCertificateConfig.CaSecret.Name, namespace)
		} else {
			ca, err = util.ReadOrGenerateCaCert(r.pki, r.Client, r.ctx, r.instance)
		}
		if err != nil {
			return err
		}

		// Generate node cert, sign it and put it into secret
		nodeSecret := corev1.Secret{}
		if err := r.Get(r.ctx, client.ObjectKey{Name: nodeSecretName, Namespace: namespace}, &nodeSecret); err != nil {
			// Generate node cert and put it into secret
			dnsNames := []string{
				clusterName,
				r.instance.Spec.General.ServiceName,
				builders.DiscoveryServiceName(r.instance),
				fmt.Sprintf("%s.%s", clusterName, namespace),
				fmt.Sprintf("%s.%s.svc", clusterName, namespace),
				fmt.Sprintf("%s.%s.svc.%s", clusterName, namespace, helpers.ClusterDnsBase()),
			}
			nodeCert, err := ca.CreateAndSignCertificate(clusterName, clusterName, dnsNames)
			if err != nil {
				r.logger.Error(err, "Failed to create node certificate", "interface", "http")
				//		r.recorder.Event(r.instance, "Warning", "Security", "Failed to create node http certifice")

				return err
			}
			nodeSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: nodeSecretName, Namespace: namespace}, Type: corev1.SecretTypeTLS, Data: nodeCert.SecretData(ca)}
			if err := ctrl.SetControllerReference(r.instance, &nodeSecret, r.Client.Scheme()); err != nil {
				return err
			}
			if err := r.Create(r.ctx, &nodeSecret); err != nil {
				r.logger.Error(err, "Failed to store node certificate in secret", "interface", "http")
				//		r.recorder.Event(r.instance, "Warning", "Security", "Failed to store node http certificate in secret")
				return err
			}
		}
		// Tell cluster controller to mount secrets
		volume := corev1.Volume{Name: "http-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: nodeSecretName}}}
		r.reconcilerContext.Volumes = append(r.reconcilerContext.Volumes, volume)
		mount := corev1.VolumeMount{Name: "http-cert", MountPath: "/usr/share/opensearch/config/tls-" + "http"}
		r.reconcilerContext.VolumeMounts = append(r.reconcilerContext.VolumeMounts, mount)
	} else {
		if tlsConfig.TlsCertificateConfig.Secret.Name == "" {
			err := errors.New("missing secret in spec")
			r.logger.Error(err, "Not all secrets for http provided")
			//		r.recorder.Event(r.instance, "Warning", "Security", "Notice - Not all secrets for http provided")
			return err
		}
		if tlsConfig.TlsCertificateConfig.CaSecret.Name == "" {
			mountFolder("http", "certs", tlsConfig.TlsCertificateConfig.Secret.Name, r.reconcilerContext)
		} else {
			mount("http", "ca", CaCertKey, tlsConfig.TlsCertificateConfig.CaSecret.Name, r.reconcilerContext)
			mount("http", "key", corev1.TLSPrivateKeyKey, tlsConfig.TlsCertificateConfig.Secret.Name, r.reconcilerContext)
			mount("http", "cert", corev1.TLSCertKey, tlsConfig.TlsCertificateConfig.Secret.Name, r.reconcilerContext)
		}
	}
	// Extend opensearch.yml
	r.reconcilerContext.AddConfig("plugins.security.ssl.http.enabled", "true")
	r.reconcilerContext.AddConfig("plugins.security.ssl.http.pemcert_filepath", fmt.Sprintf("tls-http/%s", corev1.TLSCertKey))
	r.reconcilerContext.AddConfig("plugins.security.ssl.http.pemkey_filepath", fmt.Sprintf("tls-http/%s", corev1.TLSPrivateKeyKey))
	r.reconcilerContext.AddConfig("plugins.security.ssl.http.pemtrustedcas_filepath", fmt.Sprintf("tls-http/%s", CaCertKey))
	return nil
}

func (r *TLSReconciler) providedCaCert(secretName string, namespace string) (tls.Cert, error) {
	var ca tls.Cert
	caSecret := corev1.Secret{}
	if err := r.Get(r.ctx, client.ObjectKey{Name: secretName, Namespace: namespace}, &caSecret); err != nil {
		return ca, err
	}
	data := caSecret.Data
	if _, ok := caSecret.Annotations["cert-manager.io/issuer-kind"]; ok {
		data = map[string][]byte{
			"ca.crt": caSecret.Data["tls.crt"],
			"ca.key": caSecret.Data["tls.key"],
		}
	}
	ca = r.pki.CAFromSecret(data)
	return ca, nil
}

func mount(interfaceName string, name string, filename string, secretName string, reconcilerContext *ReconcilerContext) {
	volume := corev1.Volume{Name: interfaceName + "-" + name, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secretName}}}
	reconcilerContext.Volumes = append(reconcilerContext.Volumes, volume)
	mount := corev1.VolumeMount{Name: interfaceName + "-" + name, MountPath: fmt.Sprintf("/usr/share/opensearch/config/tls-%s/%s", interfaceName, filename), SubPath: filename}
	reconcilerContext.VolumeMounts = append(reconcilerContext.VolumeMounts, mount)
}

func mountFolder(interfaceName string, name string, secretName string, reconcilerContext *ReconcilerContext) {
	volume := corev1.Volume{Name: interfaceName + "-" + name, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secretName}}}
	reconcilerContext.Volumes = append(reconcilerContext.Volumes, volume)
	mount := corev1.VolumeMount{Name: interfaceName + "-" + name, MountPath: fmt.Sprintf("/usr/share/opensearch/config/tls-%s", interfaceName)}
	reconcilerContext.VolumeMounts = append(reconcilerContext.VolumeMounts, mount)
}

func (r *TLSReconciler) DeleteResources() (ctrl.Result, error) {
	result := reconciler.CombinedResult{}
	return result.Result, result.Err
}
