package builders

import (
	"fmt"
	"strings"

	sts "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"
)

/// package that declare and build all the resources that related to the OpenSearch cluster ///

func NewSTSForNodePool(cr *opsterv1.OpenSearchCluster, node opsterv1.NodePool, volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) sts.StatefulSet {
	disk := fmt.Sprint(node.DiskSize)

	availableRoles := []string{
		"master",
		"data",
		//"data_content",
		//"data_hot",
		//"data_warm",:
		//"data_cold",
		//"data_frozen",
		"ingest",
		//"ml",
		//"remote_cluster_client",
		//"transform",
	}
	var selectedRoles []string
	for _, role := range node.Roles {
		if helpers.ContainsString(availableRoles, role) {
			selectedRoles = append(selectedRoles, role)
		}
	}

	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(disk),
				},
			},
		},
	}
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "pvc",
		MountPath: "/usr/share/opensearch/data",
	})

	clusterInitNode := helpers.CreateInitMasters(cr)
	//var vendor string
	labels := map[string]string{
		"opensearch.cluster":  cr.Name,
		"opensearch.nodepool": node.Component,
	}
	if helpers.ContainsString(selectedRoles, "master") {
		labels["opensearch.role"] = "master"
	}
	runas := int64(0)

	if cr.Spec.General.Vendor == "Op" || cr.Spec.General.Vendor == "OP" ||
		cr.Spec.General.Vendor == "Opensearch" ||
		cr.Spec.General.Vendor == "opensearch" ||
		cr.Spec.General.Vendor == "" {
		//	vendor = "opensearchproject/opensearch"
	} else {
		panic("vendor=elasticsearch not implemented")
		//vendor ="elasticsearch"
	}

	var jvm string
	if node.Jvm == "" {
		jvm = "-Xmx512M -Xms512M"
	} else {
		jvm = node.Jvm
	}
	// Supress repeated log messages about a deprecated format for the publish address
	jvm += " -Dopensearch.transport.cname_in_publish_address=true"

	probe := corev1.Probe{
		PeriodSeconds:       20,
		TimeoutSeconds:      5,
		FailureThreshold:    10,
		SuccessThreshold:    1,
		InitialDelaySeconds: 10,
		ProbeHandler:        corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.IntOrString{IntVal: cr.Spec.General.HttpPort}}},
	}

	return sts.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Spec.General.ClusterName + "-" + node.Component,
			Namespace: cr.Spec.General.ClusterName,
			Labels:    labels,
		},
		Spec: sts.StatefulSetSpec{
			Replicas: &node.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			PodManagementPolicy: "Parallel",
			UpdateStrategy:      sts.StatefulSetUpdateStrategy{Type: "RollingUpdate"},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: nil,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name:  "cluster.initial_master_nodes",
									Value: clusterInitNode,
								},
								{
									Name:  "discovery.seed_hosts",
									Value: cr.Spec.General.ServiceName,
								},
								{
									Name:  "cluster.name",
									Value: cr.Spec.General.ClusterName,
								},
								{
									Name:  "network.bind_host",
									Value: "0.0.0.0",
								},
								{
									// Make elasticsearch announce its hostname instead of IP so that certificates using the hostname can be verified
									Name:      "network.publish_host",
									ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.name"}},
								},
								{
									Name:  "OPENSEARCH_JAVA_OPTS",
									Value: jvm,
								},
								{
									Name:  "node.roles",
									Value: strings.Join(selectedRoles, ","),
								},
							},

							Name:  cr.Name,
							Image: "opensearchproject/opensearch:1.0.0",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: cr.Spec.General.HttpPort,
								},
								{
									Name:          "transport",
									ContainerPort: 9300,
								},
							},
							StartupProbe:  &probe,
							LivenessProbe: &probe,
							VolumeMounts:  volumeMounts,
						},
					},
					InitContainers: []corev1.Container{{
						Name:    "init",
						Image:   "busybox",
						Command: []string{"sh", "-c"},
						Args:    []string{"chown -R 1000:1000 /usr/share/opensearch/data"},
						SecurityContext: &corev1.SecurityContext{
							RunAsUser: &runas,
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "pvc",
								MountPath: "/usr/share/opensearch/data",
							},
						},
					},
					},
					Volumes: volumes,
					//NodeSelector:       nil,
					ServiceAccountName: cr.Spec.General.ServiceAccount,
					//	Affinity:           nil,
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{pvc},
			ServiceName:          cr.Spec.General.ServiceName + "-svc",
		},
	}
}

func NewHeadlessServiceForNodePool(cr *opsterv1.OpenSearchCluster, nodePool *opsterv1.NodePool) *corev1.Service {

	labels := map[string]string{
		"opensearch.cluster":  cr.Name,
		"opensearch.nodepool": nodePool.Component,
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Spec.General.ServiceName, nodePool.Component),
			Namespace: cr.Spec.General.ClusterName,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Protocol: "TCP",
					Port:     cr.Spec.General.HttpPort,
					TargetPort: intstr.IntOrString{
						IntVal: cr.Spec.General.HttpPort,
					},
				},
				{
					Name:     "transport",
					Protocol: "TCP",
					Port:     9300,
					TargetPort: intstr.IntOrString{
						IntVal: 9300,
						StrVal: "9300",
					},
				},
			},
			Selector: labels,
			Type:     "",
		},
	}
}

func NewServiceForCR(cr *opsterv1.OpenSearchCluster) *corev1.Service {

	labels := map[string]string{
		"opensearch.cluster": cr.Name,
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Spec.General.ServiceName,
			Namespace: cr.Spec.General.ClusterName,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Protocol: "TCP",
					Port:     cr.Spec.General.HttpPort,
					TargetPort: intstr.IntOrString{
						IntVal: cr.Spec.General.HttpPort,
					},
				},
				{
					Name:     "transport",
					Protocol: "TCP",
					Port:     9300,
					TargetPort: intstr.IntOrString{
						IntVal: 9300,
						StrVal: "9300",
					},
				},
				{
					Name:     "metrics",
					Protocol: "TCP",
					Port:     9600,
					TargetPort: intstr.IntOrString{
						IntVal: 9600,
						StrVal: "9600",
					},
				},
				{
					Name:     "rca",
					Protocol: "TCP",
					Port:     9650,
					TargetPort: intstr.IntOrString{
						IntVal: 9650,
						StrVal: "9650",
					},
				},
			},
			Selector: labels,
			Type:     "",
		},
	}
}

func NewNsForCR(cr *opsterv1.OpenSearchCluster) *corev1.Namespace {

	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: cr.Spec.General.ClusterName,
		},
	}
}

func NewCmForCR(cr *opsterv1.OpenSearchCluster) *corev1.ConfigMap {

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "opensearch-yml",
			Namespace: cr.Spec.General.ClusterName,
		},
		Data: map[string]string{
			"opensearch.yml": " plugins:\n        security:\n          allow_default_init_securityindex: true\n          allow_unsafe_democertificates: true\n          audit.type: internal_opensearch\n          authcz:\n            admin_dn:\n            - CN=kirk,OU=client,O=client,L=test, C=de\n          check_snapshot_restore_write_privileges: true\n          enable_snapshot_restore_privilege: true\n          restapi:\n            roles_enabled:\n            - all_access\n            - security_rest_api_access\n          ssl:\n            http:\n              enabled: true\n              pemcert_filepath: esnode.pem\n              pemkey_filepath: esnode-key.pem\n              pemtrustedcas_filepath: root-ca.pem\n            transport:\n              enforce_hostname_verification: false\n              pemcert_filepath: esnode.pem\n              pemkey_filepath: esnode-key.pem\n              pemtrustedcas_filepath: root-ca.pem\n          system_indices:\n            enabled: true\n            indices:\n            - .opendistro-alerting-config\n            - .opendistro-alerting-alert*\n            - .opendistro-anomaly-results*\n            - .opendistro-anomaly-detector*\n            - .opendistro-anomaly-checkpoints\n            - .opendistro-anomaly-detection-state\n            - .opendistro-reports-*\n            - .opendistro-notifications-*\n            - .opendistro-notebooks\n            - .opendistro-asynchronous-search-response*",
		},
	}
}

func URLForCluster(cr *opsterv1.OpenSearchCluster) string {
	httpPort := int32(9200)
	if cr.Spec.General.HttpPort > 0 {
		httpPort = cr.Spec.General.HttpPort
	}
	return fmt.Sprintf("https://%s.%s.svc.cluster.local:%d", cr.Spec.General.ServiceName, cr.Spec.General.ClusterName, httpPort)
}
