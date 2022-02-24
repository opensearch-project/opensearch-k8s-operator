package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	tls "opensearch.opster.io/pkg/tls"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CaCertKey = "ca.crt"
)

type TlsReconciler struct {
	client.Client
	Recorder record.EventRecorder
	logr.Logger
	Instance *opsterv1.OpenSearchCluster
}

func (r *TlsReconciler) Reconcile(controllerContext *ControllerContext) (*opsterv1.ComponentStatus, error) {
	if r.Instance.Spec.Security == nil || r.Instance.Spec.Security.Tls == nil {
		r.Logger.Info("No security specified. Not doing anything")
		return nil, nil
	}
	tlsConfig := r.Instance.Spec.Security.Tls

	if tlsConfig.Transport != nil {
		if err := r.handleTransport(tlsConfig.Transport, controllerContext); err != nil {
			return nil, err
		}
	}
	if tlsConfig.Http != nil {
		if err := r.handleHttp(tlsConfig.Http.Generate, tlsConfig.Http.CertificateConfig, controllerContext); err != nil {
			return nil, err
		}
	}

	// Temporary until securityconfig controller is working
	controllerContext.AddConfig("plugins.security.allow_unsafe_democertificates", "true")
	return nil, nil
}

func (r *TlsReconciler) handleTransport(config *opsterv1.TlsConfigTransport, controllerContext *ControllerContext) error {
	if config.Generate {
		if config.PerNode {
			if err := r.handleTransportGeneratePerNode(controllerContext); err != nil {
				return err
			}
		} else {
			if err := r.handleTransportGenerateGlobal(controllerContext); err != nil {
				return err
			}
		}
	} else {
		if err := r.handleTransportExistingCerts(config.PerNode, config.CertificateConfig, config.NodesDn, controllerContext); err != nil {
			return err
		}
	}
	return nil
}

func (r *TlsReconciler) handleTransportGenerateGlobal(controllerContext *ControllerContext) error {
	namespace := r.Instance.Spec.General.ClusterName
	clusterName := r.Instance.Spec.General.ClusterName
	caSecretName := clusterName + "-ca"
	nodeSecretName := clusterName + "-transport-cert"

	r.Logger.Info("Generating certificates", "interface", "transport")

	ca, err := r.caCert(caSecretName, namespace, clusterName)
	if err != nil {
		return err
	}

	// Generate node cert, sign it and put it into secret
	nodeSecret := corev1.Secret{}
	if err := r.Get(context.TODO(), client.ObjectKey{Name: nodeSecretName, Namespace: namespace}, &nodeSecret); err != nil {
		// Generate node cert and put it into secret
		dnsNames := []string{
			clusterName,
			fmt.Sprintf("%s.%s", clusterName, namespace),
			fmt.Sprintf("%s.%s.svc", clusterName, namespace),
			fmt.Sprintf("%s.%s.svc.cluster.local", clusterName, namespace),
		}
		nodeCert, err := ca.CreateAndSignCertificate(clusterName, clusterName, dnsNames)
		if err != nil {
			r.Logger.Error(err, "Failed to create node certificate", "interface", "transport")
			return err
		}
		nodeSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: nodeSecretName, Namespace: namespace}, Type: corev1.SecretTypeTLS, Data: nodeCert.SecretData(&ca)}
		if err := r.Create(context.TODO(), &nodeSecret); err != nil {
			r.Logger.Error(err, "Failed to store node certificate in secret", "interface", "transport")
			return err
		}
	}
	// Tell cluster controller to mount secrets
	volume := corev1.Volume{Name: "transport-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: nodeSecretName}}}
	controllerContext.Volumes = append(controllerContext.Volumes, volume)
	mount := corev1.VolumeMount{Name: "transport-cert", MountPath: "/usr/share/opensearch/config/tls-transport"}
	controllerContext.VolumeMounts = append(controllerContext.VolumeMounts, mount)
	// Extend opensearch.yml
	controllerContext.AddConfig("plugins.security.nodes_dn", fmt.Sprintf("[\"CN=%s,OU=%s\"]", clusterName, clusterName))
	controllerContext.AddConfig("plugins.security.ssl.transport.pemcert_filepath", fmt.Sprintf("tls-transport/%s", corev1.TLSCertKey))
	controllerContext.AddConfig("plugins.security.ssl.transport.pemkey_filepath", fmt.Sprintf("tls-transport/%s", corev1.TLSPrivateKeyKey))
	controllerContext.AddConfig("plugins.security.ssl.transport.pemtrustedcas_filepath", fmt.Sprintf("tls-transport/%s", CaCertKey))
	controllerContext.AddConfig("plugins.security.ssl.transport.enforce_hostname_verification", "false")
	return nil
}

func (r *TlsReconciler) handleTransportGeneratePerNode(controllerContext *ControllerContext) error {
	r.Logger.Info("Generating certificates", "interface", "transport")

	namespace := r.Instance.Spec.General.ClusterName
	clusterName := r.Instance.Spec.General.ClusterName
	caSecretName := clusterName + "-ca"
	nodeSecretName := clusterName + "-transport-cert"

	ca, err := r.caCert(caSecretName, namespace, clusterName)
	if err != nil {
		return err
	}

	nodeSecret := corev1.Secret{}
	exists := true
	if err := r.Get(context.TODO(), client.ObjectKey{Name: nodeSecretName, Namespace: namespace}, &nodeSecret); err != nil {
		nodeSecret.Data = make(map[string][]byte)
		nodeSecret.ObjectMeta = metav1.ObjectMeta{Name: nodeSecretName, Namespace: namespace}
		exists = false
	}
	nodeSecret.Data[CaCertKey] = ca.CertData()

	// Generate node cert and put it into secret
	for _, nodePool := range r.Instance.Spec.NodePools {
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
				fmt.Sprintf("%s.%s", podName, clusterName),
				fmt.Sprintf("%s.%s", clusterName, namespace),
				fmt.Sprintf("%s.%s.%s", podName, clusterName, namespace),
				fmt.Sprintf("%s.%s.svc", clusterName, namespace),
				fmt.Sprintf("%s.%s.%s.svc", podName, clusterName, namespace),
				fmt.Sprintf("%s.%s.svc.cluster.local", clusterName, namespace),
				fmt.Sprintf("%s.%s.%s.svc.cluster.local", podName, clusterName, namespace),
			}
			nodeCert, err := ca.CreateAndSignCertificate(podName, clusterName, dnsNames)
			if err != nil {
				r.Logger.Error(err, "Failed to create node certificate", "interface", "transport", "node", podName)
				return err
			}
			nodeSecret.Data[certName] = nodeCert.CertData()
			nodeSecret.Data[keyName] = nodeCert.KeyData()
		}
	}
	if exists {
		if err := r.Update(context.TODO(), &nodeSecret); err != nil {
			r.Logger.Error(err, "Failed to store node certificate in secret", "interface", "transport")
			return err
		}
	} else {
		if err := r.Create(context.TODO(), &nodeSecret); err != nil {
			r.Logger.Error(err, "Failed to store node certificate in secret", "interface", "transport")
			return err
		}
	}
	// Tell cluster controller to mount secrets
	volume := corev1.Volume{Name: "transport-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: nodeSecretName}}}
	controllerContext.Volumes = append(controllerContext.Volumes, volume)
	mount := corev1.VolumeMount{Name: "transport-cert", MountPath: "/usr/share/opensearch/config/tls-transport"}
	controllerContext.VolumeMounts = append(controllerContext.VolumeMounts, mount)

	// Extend opensearch.yml
	controllerContext.AddConfig("plugins.security.nodes_dn", fmt.Sprintf("[\"CN=*,OU=%s\"]", clusterName))
	controllerContext.AddConfig("plugins.security.ssl.transport.pemcert_filepath", "tls-transport/${HOSTNAME}.crt")
	controllerContext.AddConfig("plugins.security.ssl.transport.pemkey_filepath", "tls-transport/${HOSTNAME}.key")
	controllerContext.AddConfig("plugins.security.ssl.transport.pemtrustedcas_filepath", fmt.Sprintf("tls-transport/%s", CaCertKey))
	controllerContext.AddConfig("plugins.security.ssl.transport.enforce_hostname_verification", "true")
	return nil
}

func (r *TlsReconciler) handleTransportExistingCerts(perNode bool, certConfig opsterv1.TlsCertificateConfig, nodesDn []string, controllerContext *ControllerContext) error {
	if perNode {
		if certConfig.Secret.Name == "" {
			err := errors.New("perNode=true but secret not set")
			r.Logger.Error(err, "Secret not provided")
			return err
		}
		mountFolder("transport", "certs", certConfig.Secret.Name, controllerContext)
		// TODO: Handle separate caSecret
		// Extend opensearch.yml
		controllerContext.AddConfig("plugins.security.ssl.transport.pemcert_filepath", "tls-transport/${HOSTNAME}.crt")
		controllerContext.AddConfig("plugins.security.ssl.transport.pemkey_filepath", "tls-transport/${HOSTNAME}.key")
		controllerContext.AddConfig("plugins.security.ssl.transport.enforce_hostname_verification", "true")
	} else {
		if certConfig.Secret.Name == "" {
			err := errors.New("missing secret in spec")
			r.Logger.Error(err, "Not all secrets for transport provided")
			return err
		}
		if certConfig.CaSecret.Name == "" {
			mountFolder("transport", "certs", certConfig.Secret.Name, controllerContext)
		} else {
			mount("transport", "ca", CaCertKey, certConfig.CaSecret.Name, controllerContext)
			mount("transport", "key", corev1.TLSPrivateKeyKey, certConfig.Secret.Name, controllerContext)
			mount("transport", "cert", corev1.TLSCertKey, certConfig.Secret.Name, controllerContext)
		}
		// Extend opensearch.yml
		controllerContext.AddConfig("plugins.security.ssl.transport.pemcert_filepath", fmt.Sprintf("tls-transport/%s", corev1.TLSCertKey))
		controllerContext.AddConfig("plugins.security.ssl.transport.pemkey_filepath", fmt.Sprintf("tls-transport/%s", corev1.TLSPrivateKeyKey))
		controllerContext.AddConfig("plugins.security.ssl.transport.enforce_hostname_verification", "false")
	}
	controllerContext.AddConfig("plugins.security.ssl.transport.pemtrustedcas_filepath", fmt.Sprintf("tls-transport/%s", CaCertKey))
	dnList := strings.Join(nodesDn, "\",\"")
	controllerContext.AddConfig("plugins.security.nodes_dn", fmt.Sprintf("[\"%s\"]", dnList))
	return nil
}

func (r *TlsReconciler) handleHttp(generate bool, certConfig opsterv1.TlsCertificateConfig, controllerContext *ControllerContext) error {
	namespace := r.Instance.Spec.General.ClusterName
	clusterName := r.Instance.Spec.General.ClusterName
	caSecretName := clusterName + "-ca"
	nodeSecretName := clusterName + "-http-cert"

	if generate {
		r.Logger.Info("Generating certificates", "interface", "http")

		ca, err := r.caCert(caSecretName, namespace, clusterName)
		if err != nil {
			return err
		}

		// Generate node cert, sign it and put it into secret
		nodeSecret := corev1.Secret{}
		if err := r.Get(context.TODO(), client.ObjectKey{Name: nodeSecretName, Namespace: namespace}, &nodeSecret); err != nil {
			// Generate node cert and put it into secret
			dnsNames := []string{
				clusterName,
				fmt.Sprintf("%s.%s", clusterName, namespace),
				fmt.Sprintf("%s.%s.svc", clusterName, namespace),
				fmt.Sprintf("%s.%s.svc.cluster.local", clusterName, namespace),
			}
			nodeCert, err := ca.CreateAndSignCertificate(clusterName, clusterName, dnsNames)
			if err != nil {
				r.Logger.Error(err, "Failed to create node certificate", "interface", "http")
				return err
			}
			nodeSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: nodeSecretName, Namespace: namespace}, Type: corev1.SecretTypeTLS, Data: nodeCert.SecretData(&ca)}
			if err := r.Create(context.TODO(), &nodeSecret); err != nil {
				r.Logger.Error(err, "Failed to store node certificate in secret", "interface", "http")
				return err
			}
		}
		// Tell cluster controller to mount secrets
		volume := corev1.Volume{Name: "http-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: nodeSecretName}}}
		controllerContext.Volumes = append(controllerContext.Volumes, volume)
		mount := corev1.VolumeMount{Name: "http-cert", MountPath: "/usr/share/opensearch/config/tls-" + "http"}
		controllerContext.VolumeMounts = append(controllerContext.VolumeMounts, mount)
	} else {
		if certConfig.Secret.Name == "" {
			err := errors.New("missing secret in spec")
			r.Logger.Error(err, "Not all secrets for transport provided")
			return err
		}
		if certConfig.CaSecret.Name == "" {
			mountFolder("http", "certs", certConfig.Secret.Name, controllerContext)
		} else {
			mount("http", "ca", CaCertKey, certConfig.CaSecret.Name, controllerContext)
			mount("http", "key", corev1.TLSPrivateKeyKey, certConfig.Secret.Name, controllerContext)
			mount("http", "cert", corev1.TLSCertKey, certConfig.Secret.Name, controllerContext)
		}
	}
	// Extend opensearch.yml
	controllerContext.AddConfig("plugins.security.ssl.http.enabled", "true")
	controllerContext.AddConfig("plugins.security.ssl.http.pemcert_filepath", fmt.Sprintf("tls-http/%s", corev1.TLSCertKey))
	controllerContext.AddConfig("plugins.security.ssl.http.pemkey_filepath", fmt.Sprintf("tls-http/%s", corev1.TLSPrivateKeyKey))
	controllerContext.AddConfig("plugins.security.ssl.http.pemtrustedcas_filepath", fmt.Sprintf("tls-http/%s", CaCertKey))
	return nil
}

func (r *TlsReconciler) caCert(secretName string, namespace string, clusterName string) (tls.Cert, error) {
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

func mount(interfaceName string, name string, filename string, secretName string, controllerContext *ControllerContext) {
	volume := corev1.Volume{Name: interfaceName + "-" + name, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secretName}}}
	controllerContext.Volumes = append(controllerContext.Volumes, volume)
	mount := corev1.VolumeMount{Name: interfaceName + "-" + name, MountPath: fmt.Sprintf("/usr/share/opensearch/config/tls-%s/%s", interfaceName, filename), SubPath: filename}
	controllerContext.VolumeMounts = append(controllerContext.VolumeMounts, mount)
}

func mountFolder(interfaceName string, name string, secretName string, controllerContext *ControllerContext) {
	volume := corev1.Volume{Name: interfaceName + "-" + name, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secretName}}}
	controllerContext.Volumes = append(controllerContext.Volumes, volume)
	mount := corev1.VolumeMount{Name: interfaceName + "-" + name, MountPath: fmt.Sprintf("/usr/share/opensearch/config/tls-%s", interfaceName)}
	controllerContext.VolumeMounts = append(controllerContext.VolumeMounts, mount)
}
