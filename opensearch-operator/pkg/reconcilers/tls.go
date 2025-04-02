package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/metrics"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/tls"
	"github.com/cisco-open/operator-tools/pkg/reconciler"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type TLSReconciler struct {
	client            k8s.K8sClient
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
		client:            k8s.NewK8sClient(client, ctx, append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "tls")))...),
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
	if r.reconcileAdminCert() {
		res, err := r.handleAdminCertificate()
		return lo.FromPtrOr(res, ctrl.Result{}), err
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
	return nil
}

func (r *TLSReconciler) handleAdminCertificate() (*ctrl.Result, error) {
	// TODO: This should be refactored in the API - https://github.com/Opster/opensearch-k8s-operator/issues/569
	tlsConfig := r.instance.Spec.Security.Tls.Transport
	clusterName := r.instance.Name

	var res *ctrl.Result
	var certDN string
	if tlsConfig.Generate {
		ca, err := r.getCACert()
		if err != nil {
			return nil, err
		}
		res, err = r.createAdminSecret(ca)
		if err != nil {
			return nil, err
		}
		certDN = fmt.Sprintf("CN=admin,OU=%s", clusterName)

	} else {
		certDN = strings.Join(tlsConfig.AdminDn, "\",\"")
	}

	r.reconcilerContext.AddConfig("plugins.security.authcz.admin_dn", fmt.Sprintf("[\"%s\"]", certDN))
	return res, nil
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

func (r *TLSReconciler) adminCAName() string {
	if r.securityChangeVersion() {
		return r.instance.Spec.Security.Tls.Http.TlsCertificateConfig.CaSecret.Name
	}
	return r.instance.Spec.Security.Tls.Transport.TlsCertificateConfig.CaSecret.Name
}

func (r *TLSReconciler) reconcileAdminCert() bool {
	if r.securityChangeVersion() {
		return r.instance.Spec.Security.Tls.Http != nil && r.instance.Spec.Security.Tls.Transport != nil
	}
	return r.instance.Spec.Security.Tls.Transport != nil
}

func (r *TLSReconciler) adminCAProvided() bool {
	return r.adminCAName() != ""
}

func (r *TLSReconciler) providedCAForAdminCert() (tls.Cert, error) {
	return r.providedCaCert(
		r.adminCAName(),
		r.instance.Namespace,
	)
}

func (r *TLSReconciler) getCACert() (tls.Cert, error) {
	if r.adminCAProvided() {
		return r.providedCAForAdminCert()
	}
	return util.ReadOrGenerateCaCert(r.pki, r.client, r.instance)
}

func (r *TLSReconciler) shouldCreateAdminCert(ca tls.Cert) (bool, error) {
	secret, err := r.client.GetSecret(r.adminSecretName(), r.instance.Namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			r.logger.Info("admin cert does not exist, creating")
			return true, nil
		}
		return false, err
	}

	data, ok := secret.Data[corev1.TLSCertKey]
	if !ok {
		return true, nil
	}

	validator, err := tls.NewCertValidater(data, tls.WithExpiryThreshold(5*24*time.Hour))
	if err != nil {
		return false, err
	}

	metrics.TLSCertExpiryDays.WithLabelValues(r.instance.Name, r.instance.Namespace, r.adminSecretName()).Set(validator.DaysUntilExpiry())

	if validator.IsExpiringSoon() {
		r.logger.Info("admin cert is expiring soon, recreating")
		return true, nil
	}

	verified, err := validator.IsSignedByCA(ca)
	if err != nil {
		return false, err
	}

	if !verified {
		r.logger.Info("admin cert is not signed by CA, recreating")
	}

	return !verified, nil
}

func (r *TLSReconciler) createAdminSecret(ca tls.Cert) (*ctrl.Result, error) {
	createCert, err := r.shouldCreateAdminCert(ca)
	if err != nil {
		return nil, fmt.Errorf("failed to determine if admin cert should be created: %w", err)
	}
	if !createCert {
		return nil, nil
	}

	var adminCert tls.Cert
	var err2 error

	// Use ValidTill field if specified
	if r.instance.Spec.Security.Tls.ValidTill != "" {
		validTill, err := GenerateRFC3339DateTime(r.instance.Spec.Security.Tls.ValidTill)
		if err != nil {
			r.logger.Error(err, "Failed to parse ValidTill date", "ValidTill", r.instance.Spec.Security.Tls.ValidTill)
			return nil, err
		} else {
			adminCert, err2 = ca.CreateAndSignCertificateWithExpiry("admin", r.instance.Name, nil, validTill)
			if err2 != nil {
				r.logger.Error(err2, "Failed to create and sign certificate with expiry")
				return nil, err2
			}
		}
	} else {
		// Use default expiry
		adminCert, err2 = ca.CreateAndSignCertificate("admin", r.instance.Name, nil)
		if err2 != nil {
			r.logger.Error(err2, "Failed to create and sign certificate")
			return nil, err2
		}
	}
	adminSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.adminSecretName(),
			Namespace: r.instance.Namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: adminCert.SecretData(ca),
	}
	if err := ctrl.SetControllerReference(r.instance, adminSecret, r.client.Scheme()); err != nil {
		return nil, err
	}

	validator, err := tls.NewCertValidater(adminCert.CertData())
	if err != nil {
		return nil, err
	}
	metrics.TLSCertExpiryDays.WithLabelValues(r.instance.Name, r.instance.Namespace, r.adminSecretName()).Set(validator.DaysUntilExpiry())

	return r.client.CreateSecret(adminSecret)
}

func (r *TLSReconciler) adminSecretName() string {
	return r.instance.Name + "-admin-cert"
}

func (r *TLSReconciler) handleTransportGenerateGlobal() error {
	namespace := r.instance.Namespace
	clusterName := r.instance.Name
	nodeSecretName := clusterName + "-transport-cert"

	r.logger.Info("Generating certificates", "interface", "transport")
	// r.recorder.Event(r.instance, "Normal", "Security", "Starting to generating certificates")

	var ca tls.Cert
	var err error
	if r.instance.Spec.Security.Tls.Transport.TlsCertificateConfig.CaSecret.Name != "" {
		ca, err = r.providedCaCert(r.instance.Spec.Security.Tls.Transport.TlsCertificateConfig.CaSecret.Name, namespace)
	} else {
		ca, err = util.ReadOrGenerateCaCert(r.pki, r.client, r.instance)
	}
	if err != nil {
		return err
	}

	// Generate node cert, sign it and put it into secret
	nodeSecret, err := r.client.GetSecret(nodeSecretName, namespace)
	if err != nil {
		// Generate node cert and put it into secret
		dnsNames := []string{
			clusterName,
			fmt.Sprintf("%s.%s", clusterName, namespace),
			fmt.Sprintf("%s.%s.svc", clusterName, namespace),
			fmt.Sprintf("%s.%s.svc.%s", clusterName, namespace, helpers.ClusterDnsBase()),
		}

		var nodeCert tls.Cert

		// Use ValidTill field if specified
		if r.instance.Spec.Security.Tls.ValidTill != "" {
			validTill, err := GenerateRFC3339DateTime(r.instance.Spec.Security.Tls.ValidTill)
			if err != nil {
				r.logger.Error(err, "Failed to parse ValidTill date", "ValidTill", r.instance.Spec.Security.Tls.ValidTill)
				return err
			}
			nodeCert, err = ca.CreateAndSignCertificateWithExpiry(clusterName, clusterName, dnsNames, validTill)
			if err != nil {
				r.logger.Error(err, "Failed to create and sign certificate with expiry")
				return err
			}
		} else {
			// Use default expiry
			nodeCert, err = ca.CreateAndSignCertificate(clusterName, clusterName, dnsNames)
			if err != nil {
				r.logger.Error(err, "Failed to create and sign certificate")
				return err
			}
		}
		nodeSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: nodeSecretName, Namespace: namespace}, Type: corev1.SecretTypeTLS, Data: nodeCert.SecretData(ca)}
		if err := ctrl.SetControllerReference(r.instance, &nodeSecret, r.client.Scheme()); err != nil {
			return err
		}
		_, err = r.client.CreateSecret(&nodeSecret)
		if err != nil {
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

	for key, data := range nodeSecret.Data {
		if strings.HasSuffix(key, ".crt") && key != "ca.crt" {
			validator, err := tls.NewCertValidater(data)
			if err != nil {
				return err
			}

			// Set the metric for days until expiry
			daysUntilExpiry := validator.DaysUntilExpiry()
			metrics.TLSCertExpiryDays.WithLabelValues(clusterName, namespace, nodeSecretName).Set(daysUntilExpiry)

			// Get the exact expiry date from the certificate
			expiryTime := validator.ExpiryDate()

			// Update the status fields using the UpdateOpenSearchClusterStatus method
			key := client.ObjectKey{Name: r.instance.Name, Namespace: r.instance.Namespace}
			err = r.client.UpdateOpenSearchClusterStatus(key, func(cluster *opsterv1.OpenSearchCluster) {
				cluster.Status.TransportCertificateExpiry = metav1.NewTime(expiryTime)
			})
			if err != nil {
				return err
			}
			break
		}
	}

	return nil
}

func (r *TLSReconciler) handleTransportGeneratePerNode() error {
	r.logger.Info("Generating certificates", "interface", "transport")
	// r.recorder.Event(r.instance, "Normal", "Security", "Start to generating certificates")

	namespace := r.instance.Namespace
	clusterName := r.instance.Name
	nodeSecretName := clusterName + "-transport-cert"

	var ca tls.Cert
	var err error
	if r.instance.Spec.Security.Tls.Transport.TlsCertificateConfig.CaSecret.Name != "" {
		ca, err = r.providedCaCert(r.instance.Spec.Security.Tls.Transport.TlsCertificateConfig.CaSecret.Name, namespace)
	} else {
		ca, err = util.ReadOrGenerateCaCert(r.pki, r.client, r.instance)
	}
	if err != nil {
		return err
	}

	nodeSecret, err := r.client.GetSecret(nodeSecretName, namespace)
	exists := true
	if err != nil {
		nodeSecret.Data = make(map[string][]byte)
		nodeSecret.ObjectMeta = metav1.ObjectMeta{Name: nodeSecretName, Namespace: namespace}
		exists = false
	}
	nodeSecret.Data[CaCertKey] = ca.CertData()

	// Parse ValidTill if specified
	var validTill time.Time
	var validTillErr error
	if r.instance.Spec.Security.Tls.ValidTill != "" {
		validTill, validTillErr = GenerateRFC3339DateTime(r.instance.Spec.Security.Tls.ValidTill)
		if validTillErr != nil {
			r.logger.Error(validTillErr, "Failed to parse ValidTill date", "ValidTill", r.instance.Spec.Security.Tls.ValidTill)
			return validTillErr
		}
	}

	// Generate bootstrap pod cert
	bootstrapPodName := builders.BootstrapPodName(r.instance)
	bootsStrapCertName := fmt.Sprintf("%s.crt", bootstrapPodName)
	_, bootstrapCertExists := nodeSecret.Data[bootsStrapCertName]
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

		var nodeCert tls.Cert

		// Use ValidTill field if specified
		if r.instance.Spec.Security.Tls.ValidTill != "" {
			nodeCert, err = ca.CreateAndSignCertificateWithExpiry(bootstrapPodName, clusterName, dnsNames, validTill)
			if err != nil {
				r.logger.Error(err, "Failed to create and sign certificate with expiry")
				return err
			}
		} else {
			// Use default expiry
			nodeCert, err = ca.CreateAndSignCertificate(bootstrapPodName, clusterName, dnsNames)
			if err != nil {
				r.logger.Error(err, "Failed to create and sign certificate")
				return err
			}
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

			var nodeCert tls.Cert

			// Use ValidTill field if specified and valid
			if r.instance.Spec.Security.Tls.ValidTill != "" {
				nodeCert, err = ca.CreateAndSignCertificateWithExpiry(podName, clusterName, dnsNames, validTill)
				if err != nil {
					r.logger.Error(err, "Failed to create and sign certificate with expiry")
					return err
				}
			} else {
				// Use default expiry
				nodeCert, err = ca.CreateAndSignCertificate(podName, clusterName, dnsNames)
				if err != nil {
					r.logger.Error(err, "Failed to create and sign certificate")
					return err
				}
			}
			nodeSecret.Data[certName] = nodeCert.CertData()
			nodeSecret.Data[keyName] = nodeCert.KeyData()
		}
	}
	if exists {
		_, err = r.client.CreateSecret(&nodeSecret)
		if err != nil {
			r.logger.Error(err, "Failed to store node certificate in secret", "interface", "transport")
			return err
		}
	} else {
		if err := ctrl.SetControllerReference(r.instance, &nodeSecret, r.client.Scheme()); err != nil {
			return err
		}
		_, err = r.client.CreateSecret(&nodeSecret)
		if err != nil {
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

	for key, data := range nodeSecret.Data {
		if strings.HasSuffix(key, ".crt") && key != "ca.crt" {
			validator, err := tls.NewCertValidater(data)
			if err != nil {
				return err
			}
			// Set the metric for days until expiry
			daysUntilExpiry := validator.DaysUntilExpiry()
			metrics.TLSCertExpiryDays.WithLabelValues(clusterName, namespace, nodeSecretName).Set(daysUntilExpiry)
			// Get the exact expiry date from the certificate
			expiryTime := validator.ExpiryDate()
			// Update the status fields using the UpdateOpenSearchClusterStatus method
			key := client.ObjectKey{Name: r.instance.Name, Namespace: r.instance.Namespace}
			err = r.client.UpdateOpenSearchClusterStatus(key, func(cluster *opsterv1.OpenSearchCluster) {
				cluster.Status.TransportCertificateExpiry = metav1.NewTime(expiryTime)
			})
			if err != nil {
				return err
			}
			break
		}
	}

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
			ca, err = util.ReadOrGenerateCaCert(r.pki, r.client, r.instance)
		}
		if err != nil {
			return err
		}

		// Generate node cert, sign it and put it into secret
		nodeSecret, err := r.client.GetSecret(nodeSecretName, namespace)
		if err != nil {
			// Generate node cert and put it into secret
			dnsNames := []string{
				clusterName,
				r.instance.Spec.General.ServiceName,
				builders.DiscoveryServiceName(r.instance),
				fmt.Sprintf("%s.%s", clusterName, namespace),
				fmt.Sprintf("%s.%s.svc", clusterName, namespace),
				fmt.Sprintf("%s.%s.svc.%s", clusterName, namespace, helpers.ClusterDnsBase()),
			}

			var nodeCert tls.Cert

			// Use ValidTill field if specified
			if r.instance.Spec.Security.Tls.ValidTill != "" {
				validTill, err := GenerateRFC3339DateTime(r.instance.Spec.Security.Tls.ValidTill)
				if err != nil {
					r.logger.Error(err, "Failed to parse ValidTill date", "ValidTill", r.instance.Spec.Security.Tls.ValidTill)
					return err
				}
				nodeCert, err = ca.CreateAndSignCertificateWithExpiry(clusterName, clusterName, dnsNames, validTill)
				if err != nil {
					r.logger.Error(err, "Failed to create and sign certificate with expiry")
					return err
				}
			} else {
				// Use default expiry
				nodeCert, err = ca.CreateAndSignCertificate(clusterName, clusterName, dnsNames)
				if err != nil {
					r.logger.Error(err, "Failed to create and sign certificate")
					return err
				}
			}

			nodeSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: nodeSecretName, Namespace: namespace}, Type: corev1.SecretTypeTLS, Data: nodeCert.SecretData(ca)}
			if err := ctrl.SetControllerReference(r.instance, &nodeSecret, r.client.Scheme()); err != nil {
				return err
			}
			_, err = r.client.CreateSecret(&nodeSecret)
			if err != nil {
				r.logger.Error(err, "Failed to store node certificate in secret", "interface", "http")
				//		r.recorder.Event(r.instance, "Warning", "Security", "Failed to store node http certificate in secret")
				return err
			}
		}

		// Tell cluster controller to mount secrets
		volume := corev1.Volume{Name: "http-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: nodeSecretName}}}
		r.reconcilerContext.Volumes = append(r.reconcilerContext.Volumes, volume)
		mount := corev1.VolumeMount{Name: "http-cert", MountPath: "/usr/share/opensearch/config/tls-http"}
		r.reconcilerContext.VolumeMounts = append(r.reconcilerContext.VolumeMounts, mount)

		validator, err := tls.NewCertValidater(nodeSecret.Data["tls.crt"])
		if err != nil {
			return err
		}
		daysUntilExpiry := validator.DaysUntilExpiry()
		metrics.TLSCertExpiryDays.WithLabelValues(r.instance.Name, r.instance.Namespace, nodeSecretName).Set(daysUntilExpiry)
		// Get the exact expiry date from the certificate
		expiryTime := validator.ExpiryDate()

		// Update the status fields using the UpdateOpenSearchClusterStatus method
		key := client.ObjectKey{Name: r.instance.Name, Namespace: r.instance.Namespace}
		err = r.client.UpdateOpenSearchClusterStatus(key, func(cluster *opsterv1.OpenSearchCluster) {
			cluster.Status.HttpCertificateExpiry = metav1.NewTime(expiryTime)
		})
		if err != nil {
			return err
		}
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
	caSecret, err := r.client.GetSecret(secretName, namespace)
	if err != nil {
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

	validator, err := tls.NewCertValidater(ca.CertData())
	if err != nil {
		return ca, err
	}
	metrics.TLSCertExpiryDays.WithLabelValues(r.instance.Name, r.instance.Namespace, util.CaCertKey).Set(validator.DaysUntilExpiry())

	return ca, nil
}

func (r *TLSReconciler) DeleteResources() (ctrl.Result, error) {
	result := reconciler.CombinedResult{}
	return result.Result, result.Err
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

// Define the function to be tested
func GenerateRFC3339DateTime(input string) (time.Time, error) {
	// Check if input is empty
	if input == "" {
		return time.Time{}, fmt.Errorf("input cannot be empty")
	}

	// Define regex pattern to match valid input format
	pattern := `^(\d+)([WMY])$`
	regex := regexp.MustCompile(pattern)
	matches := regex.FindStringSubmatch(input)

	if len(matches) != 3 {
		return time.Time{}, fmt.Errorf("invalid format, expected number followed by W, M, or Y")
	}

	// Extract number and unit
	numStr := matches[1]
	unit := matches[2]

	// Parse the number
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse number: %v", err)
	}

	// Validate the number is positive
	if num <= 0 {
		return time.Time{}, fmt.Errorf("number must be positive")
	}

	// Get current time in UTC
	now := time.Now().UTC()

	// Calculate the future time based on the unit
	var futureTime time.Time
	switch unit {
	case "W":
		futureTime = now.AddDate(0, 0, num*7)
	case "M":
		futureTime = now.AddDate(0, num, 0)
	case "Y":
		futureTime = now.AddDate(num, 0, 0)
	default:
		return time.Time{}, fmt.Errorf("invalid unit, expected W, M, or Y")
	}

	// Format the result in RFC3339 format with UTC timezone
	return futureTime, nil
}
