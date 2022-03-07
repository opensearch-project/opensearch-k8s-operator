/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PhasePending = "PENDING"
	PhaseRunning = "RUNNING"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type GeneralConfig struct {

	//+kubebuilder:default="opster-cluster"
	ClusterName string `json:"clusterName,omitempty"`
	//+kubebuilder:default=9200
	HttpPort int32 `json:"httpPort,omitempty"`
	//+kubebuilder:validation:Enum=Opensearch;Op;OP;os;opensearch
	Vendor           string `json:"vendor,omitempty"`
	Version          string `json:"version,omitempty"`
	ServiceAccount   string `json:"serviceAccount,omitempty"`
	ServiceName      string `json:"serviceName"`
	SetVMMaxMapCount bool   `json:"setVMMaxMapCount,omitempty"`
}

type NodePool struct {
	Component    string              `json:"component"`
	Replicas     int32               `json:"replicas"`
	DiskSize     int32               `json:"diskSize,omitempty"`
	Cpu          int32               `json:"cpu,omitempty"`
	Memory       int32               `json:"memory,omitempty"`
	Jvm          string              `json:"jvm,omitempty"`
	Roles        []string            `json:"roles"`
	Tolerations  []corev1.Toleration `json:"tolerations,omitempty"`
	NodeSelector map[string]string   `json:"nodeSelector,omitempty"`
	Affinity     *corev1.Affinity    `json:"affinity,omitempty"`
	Persistence  *PersistenceConfig  `json:"persistence,omitempty"`
}

// PersistencConfig defines options for data persistence
type PersistenceConfig struct {
	PersistenceSource `json:",inline"`
}

type PersistenceSource struct {
	PVC      *PVCSource                   `json:"pvc,omitempty"`
	EmptyDir *corev1.EmptyDirVolumeSource `json:"emptyDir,omitempty"`
	HostPath *corev1.HostPathVolumeSource `json:"hostPath,omitempty"`
}

type PVCSource struct {
	StorageClassName string                              `json:"storageClass,omitempty"`
	AccessModes      []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}

// ConfMgmt defines which additional services will be deployed
type ConfMgmt struct {
	AutoScaler  bool `json:"autoScaler,omitempty"`
	Monitoring  bool `json:"monitoring,omitempty"`
	VerUpdate   bool `json:"VerUpdate,omitempty"`
	SmartScaler bool `json:"smartScaler,omitempty"`
}

type DashboardsConfig struct {
	Enable bool                 `json:"enable,omitempty"`
	Tls    *DashboardsTlsConfig `json:"tls,omitempty"`
}

type DashboardsTlsConfig struct {
	// Enable HTTPS for Dashboards
	Enable bool `json:"enable,omitempty"`
	// Generate certificate, if false either secret or keySecret & certSecret must be provided
	Generate bool `json:"generate,omitempty"`
	// Optional, name of a secret that contains tls.key and tls.crt data, use either this or set the separate keySecret and certSecret fields
	Secret string `json:"secret,omitempty"`
	// Optional, secret that contains the private key
	KeySecret *TlsSecret `json:"keySecret,omitempty"`
	// Optional, secret that contains the certificate for the private key, must be signed by the provided CA
	CertSecret *TlsSecret `json:"certSecret,omitempty"`
}

// Security defines options for managing the opensearch-security plugin
type Security struct {
	Tls *TlsConfig `json:"tls,omitempty"`
	// TBD: securityconfig
}

// Configure tls usage for transport and http interface
type TlsConfig struct {
	Transport *TlsConfigTransport `json:"transport,omitempty"`
	Http      *TlsConfigHttp      `json:"http,omitempty"`
}

type TlsConfigTransport struct {
	// If set to true the operator will generate a CA and certificates for the cluster to use, if false secrets with existing certificates must be supplied
	Generate bool `json:"generate,omitempty"`
	// Configure transport node certificate
	PerNode           bool                 `json:"perNode,omitempty"`
	CertificateConfig TlsCertificateConfig `json:",inline,omitempty"`
	// Allowed Certificate DNs for nodes, only used when existing certificates are provided
	NodesDn []string `json:"nodesDn,omitempty"`
	// DNs of certificates that should have admin access, mainly used for securityconfig updates via securityadmin.sh, only used when existing certificates are provided
	AdminDn []string `json:"adminDn,omitempty"`
}

type TlsConfigHttp struct {
	// If set to true the operator will generate a CA and certificates for the cluster to use, if false secrets with existing certificates must be supplied
	Generate          bool                 `json:"generate,omitempty"`
	CertificateConfig TlsCertificateConfig `json:",inline,omitempty"`
}

type TlsCertificateConfig struct {
	// Optional, name of a TLS secret that contains ca.crt, tls.key and tls.crt data. If ca.crt is in a different secret provide it via the caSecret field
	Secret corev1.LocalObjectReference `json:"secret,omitempty"`
	// Optional, secret that contains the ca certificate as ca.crt. If this and generate=true is set the existing CA cert from that secret is used to generate the node certs. In this case must contain ca.crt and ca.key fields
	CaSecret corev1.LocalObjectReference `json:"caSecret,omitempty"`
}

// Reference to a secret
type TlsSecret struct {
	SecretName string  `json:"secretName"`
	Key        *string `json:"key,omitempty"`
}

// ClusterSpec defines the desired state of OpenSearchCluster
type ClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	General    GeneralConfig    `json:"general,omitempty"`
	ConfMgmt   ConfMgmt         `json:"confMgmt,omitempty"`
	Dashboards DashboardsConfig `json:"dashboards,omitempty"`
	Security   *Security        `json:"security,omitempty"`
	NodePools  []NodePool       `json:"nodePools"`
}

// ClusterStatus defines the observed state of Es
type ClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Phase            string            `json:"phase,omitempty"`
	ComponentsStatus []ComponentStatus `json:"componentsStatus"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=os;opensearch
// Es is the Schema for the es API
type OpenSearchCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

type ComponentStatus struct {
	Component   string `json:"component,omitempty"`
	Status      string `json:"status,omitempty"`
	Description string `json:"description,omitempty"`
}

//+kubebuilder:object:root=true
// EsList contains a list of Es
type OpenSearchClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenSearchCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenSearchCluster{}, &OpenSearchClusterList{})
}
