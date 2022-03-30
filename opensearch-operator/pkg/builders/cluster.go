package builders

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"
)

/// package that declare and build all the resources that related to the OpenSearch cluster ///

const (
	ClusterLabel                     = "opster.io/opensearch-cluster"
	NodePoolLabel                    = "opster.io/opensearch-nodepool"
	securityconfigChecksumAnnotation = "securityconfig/checksum"
)

func NewSTSForNodePool(cr *opsterv1.OpenSearchCluster, node opsterv1.NodePool, volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) *appsv1.StatefulSet {
	//To make sure disksize is not passed as empty
	var disksize string
	if len(node.DiskSize) == 0 {
		disksize = "30Gi"
	} else {
		disksize = node.DiskSize
	}
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

	pvc := corev1.PersistentVolumeClaim{}
	dataVolume := corev1.Volume{}

	if node.Persistence == nil || node.Persistence.PersistenceSource.PVC != nil {
		pvc = corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "data"},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: func() []corev1.PersistentVolumeAccessMode {
					if node.Persistence == nil {
						return []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
					}
					return node.Persistence.PersistenceSource.PVC.AccessModes
				}(),
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(disksize),
					},
				},
				StorageClassName: func() *string {
					if node.Persistence == nil {
						return nil
					}
					return &node.Persistence.PVC.StorageClassName
				}(),
			},
		}
	}

	if node.Persistence != nil {
		dataVolume.Name = "data"

		if node.Persistence.PersistenceSource.HostPath != nil {
			dataVolume.VolumeSource = corev1.VolumeSource{
				HostPath: node.Persistence.PersistenceSource.HostPath,
			}
		}

		if node.Persistence.PersistenceSource.EmptyDir != nil {
			dataVolume.VolumeSource = corev1.VolumeSource{
				EmptyDir: node.Persistence.PersistenceSource.EmptyDir,
			}
		}

		volumes = append(volumes, dataVolume)
	}

	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "data",
		MountPath: "/usr/share/opensearch/data",
	})

	clusterInitNode := helpers.CreateInitMasters(cr)
	//var vendor string
	labels := map[string]string{
		ClusterLabel:  cr.Name,
		NodePoolLabel: node.Component,
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

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-" + node.Component,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &node.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			PodManagementPolicy: "Parallel",
			UpdateStrategy:      appsv1.StatefulSetUpdateStrategy{Type: "RollingUpdate"},
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
									Value: cr.Name,
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

							Name:      cr.Name,
							Image:     DockerImageForCluster(cr),
							Resources: node.Resources,
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
								Name:      "data",
								MountPath: "/usr/share/opensearch/data",
							},
						},
					},
					},
					Volumes:            volumes,
					ServiceAccountName: cr.Spec.General.ServiceAccount,
					NodeSelector:       node.NodeSelector,
					Tolerations:        node.Tolerations,
					Affinity:           node.Affinity,
				},
			},
			VolumeClaimTemplates: func() []corev1.PersistentVolumeClaim {
				if node.Persistence == nil || node.Persistence.PersistenceSource.PVC != nil {
					return []corev1.PersistentVolumeClaim{pvc}
				}
				return []corev1.PersistentVolumeClaim{}
			}(),
			ServiceName: cr.Spec.General.ServiceName,
		},
	}

	if cr.Spec.General.SetVMMaxMapCount {
		sts.Spec.Template.Spec.InitContainers = append(sts.Spec.Template.Spec.InitContainers, corev1.Container{
			Name:  "init-sysctl",
			Image: "busybox:1.27.2",
			Command: []string{
				"sysctl",
				"-w",
				"vm.max_map_count=262144",
			},
			SecurityContext: &corev1.SecurityContext{
				Privileged: pointer.Bool(true),
			},
		})
	}

	return sts
}

func DockerImageForCluster(cr *opsterv1.OpenSearchCluster) string {
	// TODO: Determine version based on CR
	return "opensearchproject/opensearch:1.2.3"
}

func NewHeadlessServiceForNodePool(cr *opsterv1.OpenSearchCluster, nodePool *opsterv1.NodePool) *corev1.Service {

	labels := map[string]string{
		ClusterLabel:  cr.Name,
		NodePoolLabel: nodePool.Component,
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Spec.General.ServiceName, nodePool.Component),
			Namespace: cr.Namespace,
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
		ClusterLabel: cr.Name,
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Spec.General.ServiceName,
			Namespace: cr.Namespace,
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

func NewNodePortService(cr *opsterv1.OpenSearchCluster) *corev1.Service {
	labels := map[string]string{
		ClusterLabel: cr.Name,
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Spec.General.ServiceName + "-exposed",
			Namespace: cr.Namespace,
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
			},
			Selector: labels,
			Type:     "NodePort",
		},
	}
}
func PortForCluster(cr *opsterv1.OpenSearchCluster) int32 {
	httpPort := int32(9200)
	if cr.Spec.General.HttpPort > 0 {
		httpPort = cr.Spec.General.HttpPort
	}
	return httpPort
}
func URLForCluster(cr *opsterv1.OpenSearchCluster) string {
	httpPort := PortForCluster(cr)
	return fmt.Sprintf("https://%s.svc.cluster.local:%d", DnsOfService(cr), httpPort)
	//return fmt.Sprintf("https://localhost:9212")
}

func DnsOfService(cr *opsterv1.OpenSearchCluster) string {
	return fmt.Sprintf("%s.%s", cr.Spec.General.ServiceName, cr.Namespace)
}

func StsName(cr *opsterv1.OpenSearchCluster, nodePool *opsterv1.NodePool) string {
	return cr.Name + "-" + nodePool.Component
}

func ReplicaHostName(currentSts appsv1.StatefulSet, repNum int32) string {
	return fmt.Sprintf("%s-%d", currentSts.ObjectMeta.Name, repNum)
}

func STSInNodePools(sts appsv1.StatefulSet, nodepools []opsterv1.NodePool) bool {
	for _, nodepool := range nodepools {
		if sts.Labels[NodePoolLabel] == nodepool.Component {
			return true
		}
	}
	return false
}

func NewSecurityconfigUpdateJob(instance *opsterv1.OpenSearchCluster, jobName string, namespace string, checksum string, adminCertName string) batchv1.Job {
	dns := DnsOfService(instance)
	adminCert := "/certs/tls.crt"
	adminKey := "/certs/tls.key"
	caCert := "/certs/ca.crt"
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	volumes = append(volumes, corev1.Volume{
		Name:         "securityconfig",
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: instance.Spec.Security.Config.SecurityconfigSecret.Name}},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "securityconfig",
		MountPath: "/securityconfig",
	})
	volumes = append(volumes, corev1.Volume{
		Name:         "admin-cert",
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: adminCertName}},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "admin-cert",
		MountPath: "/certs",
	})

	arg := "ADMIN=/usr/share/opensearch/plugins/opensearch-security/tools/securityadmin.sh;" +
		"chmod +x $ADMIN;" +
		"count=0;" +
		fmt.Sprintf("until $ADMIN -cacert %s -cert %s -key %s -cd /securityconfig/ -icl -nhnv -h %s.svc.cluster.local -p 9300 || (( count++ >= 20 )); do", caCert, adminCert, adminKey, dns) +
		"  sleep 20; " +
		"done"
	annotations := map[string]string{
		securityconfigChecksumAnnotation: checksum,
	}
	terminationGracePeriodSeconds := int64(5)
	backoffLimit := int32(0)

	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: namespace, Annotations: annotations},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Name: jobName},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{{
						Name:         "updater",
						Image:        DockerImageForCluster(instance),
						Command:      []string{"/bin/bash", "-c"},
						Args:         []string{arg},
						VolumeMounts: volumeMounts,
					}},
					Volumes:       volumes,
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
}
