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
	*ImageSpec `json:",inline,omitempty"`
	//+kubebuilder:default=9200
	HttpPort int32 `json:"httpPort,omitempty"`
	//+kubebuilder:validation:Enum=Opensearch;Op;OP;os;opensearch
	Vendor           string  `json:"vendor,omitempty"`
	Version          string  `json:"version,omitempty"`
	ServiceAccount   string  `json:"serviceAccount,omitempty"`
	ServiceName      string  `json:"serviceName"`
	SetVMMaxMapCount bool    `json:"setVMMaxMapCount,omitempty"`
	DefaultRepo      *string `json:"defaultRepo,omitempty"`
	// Extra items to add to the opensearch.yml
	AdditionalConfig map[string]string `json:"additionalConfig,omitempty"`
	// Drain data nodes controls whether to drain data notes on rolling restart operations
	DrainDataNodes bool     `json:"drainDataNodes,omitempty"`
	PluginsList    []string `json:"pluginsList,omitempty"`
	// Additional volumes to mount to all pods in the cluster
	AdditionalVolumes []AdditionalVolume `json:"additionalVolumes,omitempty"`
}

type NodePool struct {
	Component        string                      `json:"component"`
	Replicas         int32                       `json:"replicas"`
	DiskSize         string                      `json:"diskSize,omitempty"`
	Resources        corev1.ResourceRequirements `json:"resources,omitempty"`
	Jvm              string                      `json:"jvm,omitempty"`
	Roles            []string                    `json:"roles"`
	Tolerations      []corev1.Toleration         `json:"tolerations,omitempty"`
	NodeSelector     map[string]string           `json:"nodeSelector,omitempty"`
	Affinity         *corev1.Affinity            `json:"affinity,omitempty"`
	Persistence      *PersistenceConfig          `json:"persistence,omitempty"`
	AdditionalConfig map[string]string           `json:"additionalConfig,omitempty"`
	Labels           map[string]string           `json:"labels,omitempty"`
	Env              []corev1.EnvVar             `json:"env,omitempty"`
}

// PersistencConfig defines options for data persistence
type PersistenceConfig struct {
	PersistenceSource `json:","`
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

type BootstrapConfig struct {
	Resources    corev1.ResourceRequirements `json:"resources,omitempty"`
	Tolerations  []corev1.Toleration         `json:"tolerations,omitempty"`
	NodeSelector map[string]string           `json:"nodeSelector,omitempty"`
	Affinity     *corev1.Affinity            `json:"affinity,omitempty"`
	Jvm          string                      `json:"jvm,omitempty"`
}

type DashboardsConfig struct {
	*ImageSpec `json:",inline,omitempty"`
	Enable     bool                        `json:"enable,omitempty"`
	Resources  corev1.ResourceRequirements `json:"resources,omitempty"`
	Replicas   int32                       `json:"replicas"`
	Tls        *DashboardsTlsConfig        `json:"tls,omitempty"`
	Version    string                      `json:"version"`
	// Additional properties for opensearch_dashboards.yaml
	AdditionalConfig map[string]string `json:"additionalConfig,omitempty"`
	// Secret that contains fields username and password for dashboards to use to login to opensearch, must only be supplied if a custom securityconfig is provided
	OpensearchCredentialsSecret corev1.LocalObjectReference `json:"opensearchCredentialsSecret,omitempty"`
	Env                         []corev1.EnvVar             `json:"env,omitempty"`
	AdditionalVolumes           []AdditionalVolume          `json:"additionalVolumes,omitempty"`
}

type DashboardsTlsConfig struct {
	// Enable HTTPS for Dashboards
	Enable bool `json:"enable,omitempty"`
	// Generate certificate, if false secret must be provided
	Generate bool `json:"generate,omitempty"`
	// foobar
	TlsCertificateConfig `json:",omitempty"`
}

// Security defines options for managing the opensearch-security plugin
type Security struct {
	Tls    *TlsConfig      `json:"tls,omitempty"`
	Config *SecurityConfig `json:"config,omitempty"`
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
	PerNode              bool `json:"perNode,omitempty"`
	TlsCertificateConfig `json:",omitempty"`
	// Allowed Certificate DNs for nodes, only used when existing certificates are provided
	NodesDn []string `json:"nodesDn,omitempty"`
	// DNs of certificates that should have admin access, mainly used for securityconfig updates via securityadmin.sh, only used when existing certificates are provided
	AdminDn []string `json:"adminDn,omitempty"`
}

type TlsConfigHttp struct {
	// If set to true the operator will generate a CA and certificates for the cluster to use, if false secrets with existing certificates must be supplied
	Generate             bool `json:"generate,omitempty"`
	TlsCertificateConfig `json:",omitempty"`
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

type SecurityConfig struct {
	// Secret that contains the differnt yml files of the opensearch-security config (config.yml, internal_users.yml, ...)
	SecurityconfigSecret corev1.LocalObjectReference `json:"securityConfigSecret,omitempty"`
	// TLS Secret that contains a client certificate (tls.key, tls.crt, ca.crt) with admin rights in the opensearch cluster. Must be set if transport certificates are provided by user and not generated
	AdminSecret corev1.LocalObjectReference `json:"adminSecret,omitempty"`
	// Secret that contains fields username and password to be used by the operator to access the opensearch cluster for node draining. Must be set if custom securityconfig is provided.
	AdminCredentialsSecret corev1.LocalObjectReference `json:"adminCredentialsSecret,omitempty"`
}

type ImageSpec struct {
	Image            *string                       `json:"image,omitempty"`
	ImagePullPolicy  *corev1.PullPolicy            `json:"imagePullPolicy,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

type AdditionalVolume struct {
	// Name to use for the volume. Required.
	Name string `json:"name"`
	// Path in the container to mount the volume at. Required.
	Path string `json:"path"`
	// Secret to use populate the volume
	Secret *corev1.SecretVolumeSource `json:"secret,omitempty"`
	// ConfigMap to use to populate the volume
	ConfigMap *corev1.ConfigMapVolumeSource `json:"configMap,omitempty"`
	// Whether to restart the pods on content change
	RestartPods bool `json:"restartPods,omitempty"`
}

// ClusterSpec defines the desired state of OpenSearchCluster
type ClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	General    GeneralConfig    `json:"general,omitempty"`
	ConfMgmt   ConfMgmt         `json:"confMgmt,omitempty"`
	Bootstrap  BootstrapConfig  `json:"bootstrap,omitempty"`
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
	Version          string            `json:"version,omitempty"`
	Initialized      bool              `json:"initialized,omitempty"`
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

func (s ImageSpec) GetImagePullPolicy() (_ corev1.PullPolicy) {
	if p := s.ImagePullPolicy; p != nil {
		return *p
	}
	return
}

func (s ImageSpec) GetImage() string {
	if s.Image == nil {
		return ""
	}
	return *s.Image
}
