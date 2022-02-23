package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/banzaicloud/operator-tools/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"
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
	}
}

func (r *TLSReconciler) Reconcile() (ctrl.Result, error) {
	lg := log.FromContext(r.ctx)

	if r.instance.Spec.Security == nil || r.instance.Spec.Security.Tls == nil {
		lg.Info("No security specified. Not doing anything")
		return ctrl.Result{}, nil
	}

	tlsConfig := r.instance.Spec.Security.Tls
	nodesDn := tlsConfig.NodesDn

	if err := r.HandleInterface("transport", tlsConfig.Transport, &nodesDn); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.HandleInterface("http", tlsConfig.Http, &nodesDn); err != nil {
		return ctrl.Result{}, err
	}
	if len(nodesDn) > 0 {
		dnList := strings.Join(nodesDn, "\",\"")
		r.reconcilerContext.AddConfig("plugins.security.nodes_dn", fmt.Sprintf("[\"%s\"]", dnList))
	}
	// Temporary until securityconfig controller is working
	r.reconcilerContext.AddConfig("plugins.security.allow_unsafe_democertificates", "true")
	return ctrl.Result{}, nil
}

func (r *TLSReconciler) HandleInterface(name string, config *opsterv1.TlsInterfaceConfig, nodesDn *[]string) error {
	lg := log.FromContext(r.ctx)

	if config == nil {
		return nil
	}
	namespace := r.instance.Spec.General.ClusterName
	clusterName := r.instance.Spec.General.ClusterName
	ca_secret_name := clusterName + "-ca"
	node_secret_name := clusterName + "-" + name + "-cert"

	if config.Generate {
		lg.Info("Generating certificates", "interface", name)
		// Check for existing CA secret
		caSecret := corev1.Secret{}
		var ca tls.Cert
		if err := r.Get(r.ctx, client.ObjectKey{Name: ca_secret_name, Namespace: namespace}, &caSecret); err != nil {
			// Generate CA cert and put it into secret
			ca, err = tls.GenerateCA(clusterName)
			if err != nil {
				lg.Error(err, "Failed to create CA", "interface", name)
				return err
			}
			caSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ca_secret_name, Namespace: namespace}, Data: ca.SecretDataCA()}
			if err := r.Create(r.ctx, &caSecret); err != nil {
				lg.Error(err, "Failed to store CA in secret", "interface", name)
				return err
			}
		} else {
			ca = tls.CAFromSecret(caSecret.Data)
		}

		// Generate node cert, sign it and put it into secret
		nodeSecret := corev1.Secret{}
		if err := r.Get(r.ctx, client.ObjectKey{Name: node_secret_name, Namespace: namespace}, &nodeSecret); err != nil {
			// Generate node cert and put it into secret
			dnsNames := []string{
				clusterName,
				fmt.Sprintf("%s.%s", clusterName, namespace),
				fmt.Sprintf("%s.%s.svc", clusterName, namespace),
				fmt.Sprintf("%s.%s.svc.cluster.local", clusterName, namespace),
			}
			nodeCert, err := ca.CreateAndSignCertificate(clusterName, dnsNames)
			if err != nil {
				lg.Error(err, "Failed to create node certificate", "interface", name)
				return err
			}
			nodeSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: node_secret_name, Namespace: namespace}, Data: nodeCert.SecretData(&ca)}
			if err := r.Create(r.ctx, &nodeSecret); err != nil {
				lg.Error(err, "Failed to store node certificate in secret", "interface", name)
				return err
			}
		}
		// Tell cluster controller to mount secrets
		volume := corev1.Volume{Name: name + "-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: node_secret_name}}}
		r.reconcilerContext.Volumes = append(r.reconcilerContext.Volumes, volume)
		mount := corev1.VolumeMount{Name: name + "-cert", MountPath: "/usr/share/opensearch/config/tls-" + name}
		r.reconcilerContext.VolumeMounts = append(r.reconcilerContext.VolumeMounts, mount)
		if name == "transport" {
			*nodesDn = append(*nodesDn, fmt.Sprintf("CN=%s", clusterName))
		}
	} else {
		if config.CaSecret == nil || config.CertSecret == nil || config.KeySecret == nil {
			err := errors.New("missing secret in spec")
			lg.Error(err, fmt.Sprintf("Not all secrets for %s provided", name))
			return err
		}
		mount(name, "ca", "ca.crt", config.CaSecret, r.reconcilerContext)
		mount(name, "key", "tls.key", config.KeySecret, r.reconcilerContext)
		mount(name, "cert", "tls.crt", config.CertSecret, r.reconcilerContext)
	}
	// Extend opensearch.yml
	if name == "transport" {
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemcert_filepath", "tls-transport/tls.crt")
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemkey_filepath", "tls-transport/tls.key")
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.pemtrustedcas_filepath", "tls-transport/ca.crt")
		r.reconcilerContext.AddConfig("plugins.security.ssl.transport.enforce_hostname_verification", "false") // TODO: Enable with per-node certificates
	} else if name == "http" {
		r.reconcilerContext.AddConfig("plugins.security.ssl.http.enabled", "true")
		r.reconcilerContext.AddConfig("plugins.security.ssl.http.pemcert_filepath", "tls-http/tls.crt")
		r.reconcilerContext.AddConfig("plugins.security.ssl.http.pemkey_filepath", "tls-http/tls.key")
		r.reconcilerContext.AddConfig("plugins.security.ssl.http.pemtrustedcas_filepath", "tls-http/ca.crt")
	}
	return nil
}

func mount(interfaceName string, name string, filename string, secret *opsterv1.TlsSecret, reconcilerContext *ReconcilerContext) {
	volume := corev1.Volume{Name: interfaceName + "-" + name, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secret.SecretName}}}
	reconcilerContext.Volumes = append(reconcilerContext.Volumes, volume)
	secretKey := filename
	if secret.Key != nil {
		secretKey = *secret.Key
	}
	mount := corev1.VolumeMount{Name: interfaceName + "-" + name, MountPath: fmt.Sprintf("/usr/share/opensearch/config/tls-%s/%s", interfaceName, filename), SubPath: secretKey}
	reconcilerContext.VolumeMounts = append(reconcilerContext.VolumeMounts, mount)
}
