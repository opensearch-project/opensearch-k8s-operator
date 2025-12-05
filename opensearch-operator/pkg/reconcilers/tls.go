package reconcilers

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver"
	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconciler"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/tls"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type certContextType string

const (
	CertContextTransport certContextType = "transport"
	CertContextHttp      certContextType = "http"
)

type certDescription struct {
	loggingName string
	certContext certContextType
	commonName  string
	dnsNames    []string
}

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
	CaCertKey                     = "ca.crt"
	SimultaneousCertGenerationCap = 8
)

func (r *TLSReconciler) Reconcile() (ctrl.Result, error) {
	if r.instance.Spec.Security == nil || r.instance.Spec.Security.Tls == nil {
		r.logger.Info("No security specified. Not doing anything")
		return ctrl.Result{}, nil
	}

	tlsConfig := r.instance.Spec.Security.Tls

	// Handle transport TLS
	if r.isTransportTlsEnabled(tlsConfig) {
		if err := r.handleTransport(); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Handle HTTP TLS: check enabled field
	if r.isHttpTlsEnabled(tlsConfig) {
		if err := r.handleHttp(); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		r.logger.Info("HTTP TLS is disabled")
		r.reconcilerContext.AddConfig("plugins.security.ssl.http.enabled", "false")
	}

	if r.isAdminCertEnabled(tlsConfig) {
		res, err := r.handleAdminCertificate()
		return lo.FromPtrOr(res, ctrl.Result{}), err
	}

	return ctrl.Result{}, nil
}

// isTransportTlsEnabled determines if transport TLS should be enabled.
// If enabled is nil (not set): enabled by default if transport config exists.
// If enabled is true: explicitly enabled.
// If enabled is false: explicitly disabled.
func (r *TLSReconciler) isTransportTlsEnabled(config *opsterv1.TlsConfig) bool {
	if config == nil {
		return false
	}
	if config.Transport == nil {
		return false
	}
	if config.Transport.Enabled != nil {
		return *config.Transport.Enabled
	}
	// Default: enabled if transport config is provided
	return true
}

// isHttpTlsEnabled determines if HTTP TLS should be enabled.
// If enabled is nil (not set): enabled by default if HTTP config exists.
// If enabled is true: explicitly enabled.
// If enabled is false: explicitly disabled.
func (r *TLSReconciler) isHttpTlsEnabled(config *opsterv1.TlsConfig) bool {
	if config == nil {
		return false
	}
	if config.Http == nil {
		return false
	}
	if config.Http.Enabled != nil {
		return *config.Http.Enabled
	}
	// Default: enabled if HTTP config is provided
	return true
}

func (r *TLSReconciler) isAdminCertEnabled(config *opsterv1.TlsConfig) bool {
	if r.securityChangeVersion() {
		return r.isHttpTlsEnabled(config)
	}
	return r.isTransportTlsEnabled(config)
}

func (r *TLSReconciler) handleTransport() error {
	config := r.instance.Spec.Security.Tls.Transport

	if config.Generate {
		if err := r.handleTransportGenerate(); err != nil {
			return err
		}
	} else {
		if err := r.handleTransportExistingCerts(); err != nil {
			return err
		}
	}
	return nil
}

func (r *TLSReconciler) handleAdminCertificate() (*ctrl.Result, error) {
	clusterName := r.instance.Name

	var res *ctrl.Result
	var certDN string
	var shouldGenerate bool

	if r.securityChangeVersion() {
		tlsConfig := r.instance.Spec.Security.Tls.Http
		shouldGenerate = tlsConfig.Generate || (r.instance.Spec.Security.Config != nil && r.instance.Spec.Security.Config.AdminSecret.Name == "")
		if shouldGenerate {
			ca, err := r.getReferencedCaCertOrDefault(r.adminCAConfig())
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
	} else {
		tlsConfig := r.instance.Spec.Security.Tls.Transport
		shouldGenerate = tlsConfig.Generate || (r.instance.Spec.Security.Config != nil && r.instance.Spec.Security.Config.AdminSecret.Name == "")
		if shouldGenerate {
			ca, err := r.getReferencedCaCertOrDefault(r.adminCAConfig())
			if err != nil {
				return nil, err
			}

			res, err = r.createAdminSecret(ca)
			if err != nil {
				return nil, err
			}
			certDN = fmt.Sprintf("CN=admin,OU=%s", clusterName)
		} else {
			certDN = strings.Join(tlsConfig.AdminDn, "\",\"") //nolint:staticcheck
		}
	}

	r.reconcilerContext.AddConfig("plugins.security.authcz.admin_dn", fmt.Sprintf("[\"%s\"]", certDN))
	return res, nil
}

func (r *TLSReconciler) checkVersionConstraint(constraint string, defaultOnError bool, errMsg string) bool {
	versionConstraint, err := semver.NewConstraint(constraint)
	if err != nil {
		panic(err)
	}

	version, err := semver.NewVersion(r.instance.Spec.General.Version)
	if err != nil {
		r.logger.Error(err, errMsg)
		return defaultOnError
	}
	return versionConstraint.Check(version)
}

func (r *TLSReconciler) securityChangeVersion() bool {
	return r.checkVersionConstraint(
		">=2.0.0",
		true,
		"unable to parse version, assuming >= 2.0.0",
	)
}

func (r *TLSReconciler) supportsHotReload() bool {
	return r.checkVersionConstraint(
		">=2.19.1",
		false,
		"unable to parse version for hot reload check, assuming not supported",
	)
}

func (r *TLSReconciler) adminCAConfig() corev1.LocalObjectReference {
	if r.securityChangeVersion() {
		return r.instance.Spec.Security.Tls.Http.CaSecret
	}
	return r.instance.Spec.Security.Tls.Transport.CaSecret
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

	adminCert, err := ca.CreateAndSignCertificate("admin", r.instance.Name, nil, r.resolveTransportCertDuration())
	if err != nil {
		r.logger.Error(err, "Failed to create admin certificate", "interface", "transport")
		r.recorder.AnnotatedEventf(
			r.instance,
			map[string]string{"cluster-name": r.instance.GetName()},
			"Warning",
			"Security",
			"Failed to create admin certificate",
		)
		return nil, err
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
	return r.client.CreateSecret(adminSecret)
}

func (r *TLSReconciler) adminSecretName() string {
	return r.instance.Name + "-admin-cert"
}

func (r *TLSReconciler) handleTransportGenerate() error {
	namespace := r.instance.Namespace
	clusterName := r.instance.Name
	nodeSecretName := clusterName + "-transport-cert"
	config := r.instance.Spec.Security.Tls.Transport
	generatePerNode := config.PerNode

	ca, err := r.getReferencedCaCertOrDefault(config.CaSecret)
	if err != nil {
		return err
	}

	r.logger.Info("Reconciling certificates", "interface", "transport")
	// r.recorder.Event(r.instance, "Normal", "Security", "Starting to generate certificates")

	nodeSecret, err := r.client.GetSecret(nodeSecretName, namespace)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			r.logger.Error(err, "Failed to get secret for transport certificate(s)")
			return err
		}

		nodeSecret.ObjectMeta = metav1.ObjectMeta{Name: nodeSecretName, Namespace: namespace}
		if generatePerNode {
			nodeSecret.Data = make(map[string][]byte)
		} else {
			nodeSecret.Type = corev1.SecretTypeTLS
		}

		if err := ctrl.SetControllerReference(r.instance, &nodeSecret, r.client.Scheme()); err != nil {
			return err
		}
	}

	if !generatePerNode {
		newCertData, err := r.generateNewCertIfNeeded(
			ca,
			certDescription{
				loggingName: "global",
				certContext: CertContextTransport,
				commonName:  clusterName,
				dnsNames: []string{
					clusterName,
					fmt.Sprintf("%s.%s", clusterName, namespace),
					fmt.Sprintf("%s.%s.svc", clusterName, namespace),
					fmt.Sprintf("%s.%s.svc.%s", clusterName, namespace, helpers.ClusterDnsBase()),
				},
			},
			nodeSecret.Data[corev1.TLSCertKey],
		)
		if err != nil {
			return err
		}
		if newCertData != nil {
			nodeSecret.Data = newCertData.SecretData(ca)
		}

	} else {
		if nodeSecret.Data == nil {
			// covers both the case where nodeSecret is new, or nodeSecret existed
			// but was nil for some unknown reason (maybe a past failure)
			nodeSecret.Data = make(map[string][]byte)
		}
		nodeSecret.Data[CaCertKey] = ca.CertData()

		if err := r.generateBootstrapCertIfNeeded(ca, &nodeSecret); err != nil {
			return err
		}

		eg, _ := errgroup.WithContext(r.client.Context())
		eg.SetLimit(min(SimultaneousCertGenerationCap, runtime.GOMAXPROCS(0)))

		secretMutex := sync.Mutex{}

		// Generate node cert and put it into secret
		for _, nodePool := range r.instance.Spec.NodePools {
			for i := 0; i < int(nodePool.Replicas); i++ {
				podName := fmt.Sprintf("%s-%s-%d", clusterName, nodePool.Component, i)
				certName := fmt.Sprintf("%s.crt", podName)
				keyName := fmt.Sprintf("%s.key", podName)
				secretMutex.Lock()
				certData := nodeSecret.Data[certName]
				_, keyExists := nodeSecret.Data[keyName]
				secretMutex.Unlock()
				if certData != nil && !keyExists {
					r.logger.Info("Node certificate exists but has no key, forcing regeneration",
						"interface", "transport", "node", podName)
					certData = nil
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
					fmt.Sprintf("%s.%s.%s.svc.%s", podName, clusterName, namespace,
						helpers.ClusterDnsBase()),
				}

				eg.Go(func() error {
					newCertData, err := r.generateNewCertIfNeeded(
						ca,
						certDescription{
							loggingName: podName,
							certContext: CertContextTransport,
							commonName:  podName,
							dnsNames:    dnsNames,
						},
						certData,
					)
					if err != nil {
						return err
					}
					if newCertData != nil {
						secretMutex.Lock()
						nodeSecret.Data[certName] = newCertData.CertData()
						nodeSecret.Data[keyName] = newCertData.KeyData()
						secretMutex.Unlock()
					}
					return nil
				})
			}
		}

		err := eg.Wait()
		if err != nil {
			r.logger.Error(err, "Not all required certificates could be created")
			return err
		}
	}

	_, err = r.client.CreateSecret(&nodeSecret)
	if err != nil {
		r.logger.Error(err, "Failed to store node certificate(s) in secret", "interface", "transport")
		return err
	}

	// Tell cluster controller to mount secrets
	volume := corev1.Volume{Name: "transport-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: nodeSecretName}}}
	r.reconcilerContext.Volumes = append(r.reconcilerContext.Volumes, volume)
	mount := corev1.VolumeMount{Name: "transport-cert", MountPath: "/usr/share/opensearch/config/tls-transport"}
	r.reconcilerContext.VolumeMounts = append(r.reconcilerContext.VolumeMounts, mount)

	// Extend opensearch.yml
	if generatePerNode {
		r.reconcilerContext.AddConfig("plugins.security.nodes_dn", fmt.Sprintf("[\"CN=%s-*,OU=%s\"]", clusterName, clusterName))
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemcert_filepath", "tls-transport/${HOSTNAME}.crt")
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemkey_filepath", "tls-transport/${HOSTNAME}.key")
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.enforce_hostname_verification", "true")
	} else {
		r.reconcilerContext.AddConfig("plugins.security.nodes_dn", fmt.Sprintf("[\"CN=%s,OU=%s\"]", clusterName, clusterName))
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemcert_filepath", fmt.Sprintf("tls-transport/%s", corev1.TLSCertKey))
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemkey_filepath", fmt.Sprintf("tls-transport/%s", corev1.TLSPrivateKeyKey))
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.enforce_hostname_verification", "false")
	}

	r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemtrustedcas_filepath", fmt.Sprintf("tls-transport/%s", CaCertKey))

	return nil
}

func (r *TLSReconciler) generateBootstrapCertIfNeeded(
	ca tls.Cert,
	nodeSecret *corev1.Secret,
) error {
	namespace := r.instance.Namespace
	clusterName := r.instance.Name

	// Generate bootstrap pod cert
	bootstrapPodName := builders.BootstrapPodName(r.instance)
	_, bootstrapCertExists := nodeSecret.Data[fmt.Sprintf("%s.crt", bootstrapPodName)]
	_, bootstrapKeyExists := nodeSecret.Data[fmt.Sprintf("%s.key", bootstrapPodName)]

	if !r.instance.Status.Initialized && (!bootstrapCertExists || !bootstrapKeyExists) {
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
		nodeCert, err := ca.CreateAndSignCertificate(bootstrapPodName, clusterName, dnsNames, r.resolveTransportCertDuration())
		if err != nil {
			r.logger.Error(err, "Failed to create node certificate", "interface", "transport", "node", bootstrapPodName)
			//	r.recorder.Event(r.instance, "Normal", "Security", "Created transport certificates")
			return err
		}
		//	r.recorder.Event(r.instance, "Normal", "Security", "Created transport certificates")
		nodeSecret.Data[fmt.Sprintf("%s.crt", bootstrapPodName)] = nodeCert.CertData()
		nodeSecret.Data[fmt.Sprintf("%s.key", bootstrapPodName)] = nodeCert.KeyData()
	}
	return nil
}

func (r *TLSReconciler) generateNewCertIfNeeded(
	ca tls.Cert,
	cd certDescription,
	existingCertData []byte,
) (tls.Cert, error) {
	clusterName := r.instance.Name

	if existingCertData != nil && !r.certShouldBeRenewed(cd, existingCertData) {
		return nil, nil
	}

	var certDuration time.Duration
	switch cd.certContext {
	case CertContextHttp:
		certDuration = r.resolveHttpCertDuration()
	case CertContextTransport:
		certDuration = r.resolveTransportCertDuration()
	default:
		panic("unrecognized certDescription.certContext value")
	}

	nodeCert, err := ca.CreateAndSignCertificate(cd.commonName, clusterName,
		cd.dnsNames, certDuration)
	if err != nil {
		r.logger.Error(err, "Failed to create certificate", "interface",
			cd.certContext, "node", cd.loggingName)
		//		r.recorder.Event(r.instance, "Warning", "Security", "Failed to create node http certifice")
		return nil, err
	}
	return nodeCert, nil
}

func (r *TLSReconciler) certShouldBeRenewed(cd certDescription, existingCertData []byte) bool {
	namespace := r.instance.Namespace
	clusterName := r.instance.Name

	var renewBeforeExpirationDays int
	switch cd.certContext {
	case CertContextTransport:
		renewBeforeExpirationDays = r.instance.Spec.Security.Tls.Transport.RotateDaysBeforeExpiry
	case CertContextHttp:
		renewBeforeExpirationDays = r.instance.Spec.Security.Tls.Http.RotateDaysBeforeExpiry
	default:
		panic("unrecognized certDescription.certContext value")
	}

	daysRemaining, err := getDaysRemainingFromCertificate(existingCertData)
	if err != nil {
		r.logger.Error(err, "Failed to parse certificate for expiry date - not renewing", "interface",
			cd.certContext, "node", cd.loggingName)
		return false
	}

	helpers.TlsCertificateDaysRemaining.WithLabelValues(namespace,
		clusterName, string(cd.certContext), cd.loggingName).Set(float64(daysRemaining))

	return (renewBeforeExpirationDays > 0 && daysRemaining < renewBeforeExpirationDays)
}

func (r *TLSReconciler) handleTransportExistingCerts() error {
	tlsConfig := r.instance.Spec.Security.Tls.Transport
	if tlsConfig.Secret.Name == "" {
		err := errors.New("missing secret in spec")
		r.logger.Error(err, "Not all secrets for transport provided")
		//		r.recorder.Event(r.instance, "Warning", "Security", "Notice - Not all secrets for transport provided")
		return err
	}

	if tlsConfig.PerNode {
		mountFolder("transport", "certs", tlsConfig.Secret.Name, r.reconcilerContext)
		// Extend opensearch.yml
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemcert_filepath", "tls-transport/${HOSTNAME}.crt")
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemkey_filepath", "tls-transport/${HOSTNAME}.key")
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.enforce_hostname_verification", "true")
	} else {
		// Implement new mounting logic based on CaSecret.Name configuration
		switch name := tlsConfig.CaSecret.Name; name {
		case "":
			// If CaSecret.Name is empty, mount Secret.Name as a directory
			mountFolder("transport", "certs", tlsConfig.Secret.Name, r.reconcilerContext)
		case tlsConfig.Secret.Name:
			// If CaSecret.Name is same as Secret.Name, mount only Secret.Name as a directory
			mountFolder("transport", "certs", tlsConfig.Secret.Name, r.reconcilerContext)
		default:
			// If CaSecret.Name is different from Secret.Name, mount both secrets as directories
			// Mount Secret.Name as tls-transport/
			mountFolder("transport", "certs", tlsConfig.Secret.Name, r.reconcilerContext)
			// Mount CaSecret.Name as tls-transport-ca/
			mountFolder("transport", "ca", tlsConfig.CaSecret.Name, r.reconcilerContext)
		}

		// Extend opensearch.yml with appropriate file paths based on mounting logic
		if tlsConfig.CaSecret.Name == "" || tlsConfig.CaSecret.Name == tlsConfig.Secret.Name {
			// Single secret mounted as directory
			r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemcert_filepath", fmt.Sprintf("tls-transport/%s", corev1.TLSCertKey))
			r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemkey_filepath", fmt.Sprintf("tls-transport/%s", corev1.TLSPrivateKeyKey))
			r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemtrustedcas_filepath", fmt.Sprintf("tls-transport/%s", CaCertKey))
		} else {
			// Separate secrets mounted as directories
			r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemcert_filepath", fmt.Sprintf("tls-transport/%s", corev1.TLSCertKey))
			r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemkey_filepath", fmt.Sprintf("tls-transport/%s", corev1.TLSPrivateKeyKey))
			r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemtrustedcas_filepath", fmt.Sprintf("tls-transport-ca/%s", CaCertKey))
		}
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.enforce_hostname_verification", "false")

		// Enable hot reload if configured and version supports it
		if tlsConfig.EnableHotReload && r.supportsHotReload() {
			r.reconcilerContext.AddConfig("plugins.security.ssl.certificates_hot_reload.enabled", "true")
		}
	}
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
		r.logger.Info("Reconciling certificates", "interface", "http")

		ca, err := r.getReferencedCaCertOrDefault(tlsConfig.CaSecret)
		if err != nil {
			return err
		}

		// Generate node cert, sign it and put it into secret
		nodeSecret, err := r.client.GetSecret(nodeSecretName, namespace)
		if err != nil {
			if !k8serrors.IsNotFound(err) {
				r.logger.Error(err, "Failed to get secret for http certificate")
				return err
			}

			nodeSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: nodeSecretName, Namespace: namespace}, Type: corev1.SecretTypeTLS}
			if err := ctrl.SetControllerReference(r.instance, &nodeSecret, r.client.Scheme()); err != nil {
				return err
			}
		}

		// Generate node cert and put it into secret
		// Build default DNS names
		dnsNames := []string{
			clusterName,
			r.instance.Spec.General.ServiceName,
			builders.DiscoveryServiceName(r.instance),
			fmt.Sprintf("%s.%s", clusterName, namespace),
			fmt.Sprintf("%s.%s.svc", clusterName, namespace),
			fmt.Sprintf("%s.%s.svc.%s", clusterName, namespace, helpers.ClusterDnsBase()),
		}

		// Prepend custom FQDN if provided
		if tlsConfig.CustomFQDN != nil && *tlsConfig.CustomFQDN != "" {
			dnsNames = append([]string{*tlsConfig.CustomFQDN}, dnsNames...)
		}

		nodeCert, err := r.generateNewCertIfNeeded(
			ca,
			certDescription{
				loggingName: "global",
				certContext: CertContextHttp,
				commonName:  clusterName,
				dnsNames:    dnsNames,
			},
			nodeSecret.Data[corev1.TLSCertKey],
		)

		if err != nil {
			return err
		}
		if nodeCert != nil {
			nodeSecret.Data = nodeCert.SecretData(ca)
		}

		_, err = r.client.CreateSecret(&nodeSecret)
		if err != nil {
			r.logger.Error(err, "Failed to store node certificate in secret", "interface", "http")
			//		r.recorder.Event(r.instance, "Warning", "Security", "Failed to store node http certificate in secret")
			return err
		}

		// Tell cluster controller to mount secrets
		volume := corev1.Volume{Name: "http-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: nodeSecretName}}}
		r.reconcilerContext.Volumes = append(r.reconcilerContext.Volumes, volume)
		mount := corev1.VolumeMount{Name: "http-cert", MountPath: "/usr/share/opensearch/config/tls-" + "http"}
		r.reconcilerContext.VolumeMounts = append(r.reconcilerContext.VolumeMounts, mount)
	} else {
		if tlsConfig.Secret.Name == "" {
			err := errors.New("missing secret in spec")
			r.logger.Error(err, "Not all secrets for http provided")
			//		r.recorder.Event(r.instance, "Warning", "Security", "Notice - Not all secrets for http provided")
			return err
		}

		// Implement new mounting logic based on CaSecret.Name configuration
		switch name := tlsConfig.CaSecret.Name; name {
		case "":
			// If CaSecret.Name is empty, mount Secret.Name as a directory
			mountFolder("http", "certs", tlsConfig.Secret.Name, r.reconcilerContext)
		case tlsConfig.Secret.Name:
			// If CaSecret.Name is same as Secret.Name, mount only Secret.Name as a directory
			mountFolder("http", "certs", tlsConfig.Secret.Name, r.reconcilerContext)
		default:
			// If CaSecret.Name is different from Secret.Name, mount both secrets as directories
			// Mount Secret.Name as tls-http/
			mountFolder("http", "certs", tlsConfig.Secret.Name, r.reconcilerContext)
			// Mount CaSecret.Name as tls-http-ca/
			mountFolder("http", "ca", tlsConfig.CaSecret.Name, r.reconcilerContext)
		}
	}
	// Extend opensearch.yml with appropriate file paths based on mounting logic
	r.reconcilerContext.AddConfig("plugins.security.ssl.http.enabled", "true")

	// Set certificate file paths based on mounting configuration
	if tlsConfig.CaSecret.Name == "" || tlsConfig.CaSecret.Name == tlsConfig.Secret.Name {
		// Single secret mounted as directory
		r.reconcilerContext.AddConfig("plugins.security.ssl.http.pemcert_filepath", fmt.Sprintf("tls-http/%s", corev1.TLSCertKey))
		r.reconcilerContext.AddConfig("plugins.security.ssl.http.pemkey_filepath", fmt.Sprintf("tls-http/%s", corev1.TLSPrivateKeyKey))
		r.reconcilerContext.AddConfig("plugins.security.ssl.http.pemtrustedcas_filepath", fmt.Sprintf("tls-http/%s", CaCertKey))
	} else {
		// Separate secrets mounted as directories
		r.reconcilerContext.AddConfig("plugins.security.ssl.http.pemcert_filepath", fmt.Sprintf("tls-http/%s", corev1.TLSCertKey))
		r.reconcilerContext.AddConfig("plugins.security.ssl.http.pemkey_filepath", fmt.Sprintf("tls-http/%s", corev1.TLSPrivateKeyKey))
		r.reconcilerContext.AddConfig("plugins.security.ssl.http.pemtrustedcas_filepath", fmt.Sprintf("tls-http-ca/%s", CaCertKey))
	}

	// Enable hot reload if configured and version supports it
	if tlsConfig.EnableHotReload && r.supportsHotReload() {
		r.reconcilerContext.AddConfig("plugins.security.ssl.certificates_hot_reload.enabled", "true")
	}
	return nil
}

func (r *TLSReconciler) getReferencedCaCertOrDefault(
	secretReference corev1.LocalObjectReference,
) (tls.Cert, error) {
	if secretReference.Name == "" {
		return util.ReadOrGenerateCaCert(r.pki, r.client, r.instance)
	}

	var ca tls.Cert
	caSecret, err := r.client.GetSecret(secretReference.Name, r.instance.Namespace)
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
	return ca, nil
}

func mountFolder(interfaceName string, name string, secretName string, reconcilerContext *ReconcilerContext) {
	volume := corev1.Volume{Name: interfaceName + "-" + name, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secretName}}}
	reconcilerContext.Volumes = append(reconcilerContext.Volumes, volume)

	var mountPath string
	if name == "ca" {
		mountPath = fmt.Sprintf("/usr/share/opensearch/config/tls-%s-ca", interfaceName)
	} else {
		mountPath = fmt.Sprintf("/usr/share/opensearch/config/tls-%s", interfaceName)
	}

	mount := corev1.VolumeMount{Name: interfaceName + "-" + name, MountPath: mountPath}
	reconcilerContext.VolumeMounts = append(reconcilerContext.VolumeMounts, mount)
}

func (r *TLSReconciler) DeleteResources() (ctrl.Result, error) {
	result := reconciler.CombinedResult{}
	return result.Result, result.Err
}

func getDaysRemainingFromCertificate(data []byte) (int, error) {
	der, _ := pem.Decode(data)
	if der == nil {
		return -1, fmt.Errorf("failed to decode valid PEM from provided certificate data")
	}
	cert, err := x509.ParseCertificate(der.Bytes)
	if err != nil {
		return -1, err
	}
	daysRemaining := int(time.Until(cert.NotAfter).Hours() / 24)
	return daysRemaining, nil
}

func (r *TLSReconciler) resolveTransportCertDuration() time.Duration {
	if r.instance.Spec.Security != nil && r.instance.Spec.Security.Tls != nil && r.instance.Spec.Security.Tls.Transport != nil {
		if r.instance.Spec.Security.Tls.Transport.Duration != nil {
			return r.instance.Spec.Security.Tls.Transport.Duration.Duration
		}
	}
	return 365 * 24 * time.Hour
}

func (r *TLSReconciler) resolveHttpCertDuration() time.Duration {
	if r.instance.Spec.Security != nil && r.instance.Spec.Security.Tls != nil && r.instance.Spec.Security.Tls.Http != nil {
		if r.instance.Spec.Security.Tls.Http.Duration != nil {
			return r.instance.Spec.Security.Tls.Http.Duration.Duration
		}
	}
	return 365 * 24 * time.Hour
}
