package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	tls "opensearch.opster.io/pkg/tls"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TlsReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Instance *opsterv1.OpenSearchCluster
}

func (r *TlsReconciler) Reconcile(controllerContext *ControllerContext) (*opsterv1.ComponentStatus, error) {
	if r.Instance.Spec.Security == nil || r.Instance.Spec.Security.Tls == nil {
		fmt.Println("No security specified. Not doing anything")
		return nil, nil
	}
	tlsConfig := r.Instance.Spec.Security.Tls

	if err := r.HandleInterface("transport", tlsConfig.Transport, controllerContext); err != nil {
		return nil, err
	}
	if err := r.HandleInterface("http", tlsConfig.Http, controllerContext); err != nil {
		return nil, err
	}
	// Temporary until securityconfig controller is working
	controllerContext.OpenSearchConfig = append(controllerContext.OpenSearchConfig, "plugins.security.allow_unsafe_democertificates: true")
	return nil, nil
}

func (r *TlsReconciler) HandleInterface(name string, config *opsterv1.TlsInterfaceConfig, controllerContext *ControllerContext) error {
	fmt.Printf("Handling %s\n", name)
	namespace := r.Instance.Spec.General.ClusterName
	clusterName := r.Instance.Spec.General.ClusterName
	ca_secret_name := clusterName + "-ca"
	node_secret_name := clusterName + "-" + name + "-cert"

	if config.Generate {
		// Check for existing CA secret
		caSecret := corev1.Secret{}
		var ca tls.Cert
		if err := r.Get(context.TODO(), client.ObjectKey{Name: ca_secret_name, Namespace: namespace}, &caSecret); err != nil {
			// Generate CA cert and put it into secret
			ca, err = tls.GenerateCA(clusterName)
			if err != nil {
				fmt.Println("Failed to create CA")
				return err
			}
			caSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ca_secret_name, Namespace: namespace}, Data: ca.SecretDataCA()}
			if err := r.Create(context.TODO(), &caSecret); err != nil {
				fmt.Println("Failed to store CA in secret")
				return err
			}
		} else {
			fmt.Println("Using existing CA secret")
			ca = tls.CAFromSecret(caSecret.Data)
		}

		// Generate node cert, sign it and put it into secret
		nodeSecret := corev1.Secret{}
		if err := r.Get(context.TODO(), client.ObjectKey{Name: node_secret_name, Namespace: namespace}, &nodeSecret); err != nil {
			fmt.Printf("Generating certificate for %s\n", name)
			// Generate node cert and put it into secret
			dnsNames := []string{
				clusterName,
				fmt.Sprintf("%s.%s", clusterName, namespace),
				fmt.Sprintf("%s.%s.svc", clusterName, namespace),
				fmt.Sprintf("%s.%s.svc.cluster.local", clusterName, namespace),
			}
			nodeCert, err := ca.CreateAndSignCertificate(clusterName, dnsNames)
			if err != nil {
				fmt.Println("Failed to create node certificate")
				return err
			}
			nodeSecret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: node_secret_name, Namespace: namespace}, Data: nodeCert.SecretData(&ca)}
			if err := r.Create(context.TODO(), &nodeSecret); err != nil {
				fmt.Println("Failed to store node certificate in secret")
				return err
			}
		}
		// Extend opensearch.yml
		if name == "transport" {
			controllerContext.OpenSearchConfig = append(controllerContext.OpenSearchConfig, "plugins.security.ssl.transport.pemcert_filepath: tls-transport/tls.crt\n"+
				"plugins.security.ssl.transport.pemkey_filepath: tls-transport/tls.key\n"+
				"plugins.security.ssl.transport.pemtrustedcas_filepath: tls-transport/ca.crt\n"+
				"plugins.security.ssl.transport.enforce_hostname_verification: false\n"+
				fmt.Sprintf("plugins.security.nodes_dn: [\"CN=%s\"]\n", clusterName))
		} else if name == "http" {
			controllerContext.OpenSearchConfig = append(controllerContext.OpenSearchConfig, "plugins.security.ssl.http.enabled: true\n"+
				"plugins.security.ssl.http.pemcert_filepath: tls-http/tls.crt\n"+
				"plugins.security.ssl.http.pemkey_filepath: tls-http/tls.key\n"+
				"plugins.security.ssl.http.pemtrustedcas_filepath: tls-http/ca.crt\n")
		}
		// Tell cluster controller to mount secrets
		volume := corev1.Volume{Name: name + "-cert", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: node_secret_name}}}
		controllerContext.Volumes = append(controllerContext.Volumes, volume)
		mount := corev1.VolumeMount{Name: name + "-cert", MountPath: "/usr/share/opensearch/config/tls-" + name}
		controllerContext.VolumeMounts = append(controllerContext.VolumeMounts, mount)
	} else {
		// TODO
		panic("Using existing certs from secrets not yet supported")
	}
	return nil
}
