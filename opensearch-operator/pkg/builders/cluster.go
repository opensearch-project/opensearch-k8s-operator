package builders

import (
	"context"
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/// package that declare and build all the resources that related to the OpenSearch cluster ///

const (
	ClusterLabel                     = "opster.io/opensearch-cluster"
	NodePoolLabel                    = "opster.io/opensearch-nodepool"
	ConfigurationChecksumAnnotation  = "opster.io/config"
	securityconfigChecksumAnnotation = "securityconfig/checksum"
)

func NewSTSForNodePool(
	username string,
	cr *opsterv1.OpenSearchCluster,
	node opsterv1.NodePool,
	configChecksum string,
	volumes []corev1.Volume,
	volumeMounts []corev1.VolumeMount,
	extraConfig map[string]string,
) *appsv1.StatefulSet {
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
		"data_content",
		"data_hot",
		"data_warm",
		"data_cold",
		"data_frozen",
		"ingest",
		"ml",
		"remote_cluster_client",
		"transform",
		"cluster_manager",
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
		mode := corev1.PersistentVolumeFilesystem
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
				VolumeMode: &mode,
			},
		}
	}

	if node.Persistence != nil {
		dataVolume.Name = "data"

		if node.Persistence.PersistenceSource.HostPath != nil {
			dataVolume.VolumeSource = corev1.VolumeSource{
				HostPath: node.Persistence.PersistenceSource.HostPath,
			}
			volumes = append(volumes, dataVolume)
		}

		if node.Persistence.PersistenceSource.EmptyDir != nil {
			dataVolume.VolumeSource = corev1.VolumeSource{
				EmptyDir: node.Persistence.PersistenceSource.EmptyDir,
			}
			volumes = append(volumes, dataVolume)
		}
	}

	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "data",
		MountPath: "/usr/share/opensearch/data",
	})

	//var vendor string
	labels := map[string]string{
		ClusterLabel:  cr.Name,
		NodePoolLabel: node.Component,
	}
	annotations := map[string]string{
		ConfigurationChecksumAnnotation: configChecksum,
	}

	if helpers.ContainsString(selectedRoles, "master") {
		labels["opensearch.role"] = "master"
	}

	// cr.Spec.NodePool.labels
	for k, v := range node.Labels {
		labels[k] = v
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

	// Because the http endpoint requires auth we need to do it as a curl script
	httpPort := PortForCluster(cr)
	curlCmd := "curl -k -u \"${OPENSEARCH_USER}:${OPENSEARCH_PASSWORD}\" --silent --fail https://localhost:" + fmt.Sprint(httpPort)
	readinessProbe := corev1.Probe{
		InitialDelaySeconds: 30,
		PeriodSeconds:       30,
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"/bin/bash",
					"-c",
					curlCmd,
				},
			},
		},
	}

	image := helpers.ResolveImage(cr, &node)

	var mainCommand []string
	var com string
	com = "yes | ./bin/opensearch-plugin install"
	if len(cr.Spec.General.PluginsList) > 0 {
		mainCommand = append(mainCommand, "/bin/bash", "-c")
		for index, plugin := range cr.Spec.General.PluginsList {
			fmt.Println(index, plugin)
			com = com + " '" + strings.Replace(plugin, "'", "\\'", -1) + "'"
		}

		com = com + " ; ./opensearch-docker-entrypoint.sh"
		mainCommand = append(mainCommand, com)
	} else {
		mainCommand = []string{"/bin/bash", "-c", "./opensearch-docker-entrypoint.sh"}
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
			PodManagementPolicy: appsv1.OrderedReadyPodManagement,
			UpdateStrategy: func() appsv1.StatefulSetUpdateStrategy {
				if helpers.ContainsString(selectedRoles, "data") {
					return appsv1.StatefulSetUpdateStrategy{
						Type: appsv1.OnDeleteStatefulSetStrategyType,
					}
				}
				return appsv1.StatefulSetUpdateStrategy{
					Type: appsv1.RollingUpdateStatefulSetStrategyType,
				}
			}(),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name:  "cluster.initial_master_nodes",
									Value: BootstrapPodName(cr),
								},
								{
									Name:  "discovery.seed_hosts",
									Value: DiscoveryServiceName(cr),
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
								{
									Name:  "http.port",
									Value: fmt.Sprint(cr.Spec.General.HttpPort),
								},
								{
									Name:  "OPENSEARCH_USER",
									Value: username,
								},
								{
									Name: "OPENSEARCH_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: fmt.Sprintf("%s-admin-password", cr.Name),
											},
											Key: "password",
										},
									},
								},
							},
							Name:            "opensearch",
							Command:         mainCommand,
							Image:           image.GetImage(),
							ImagePullPolicy: image.GetImagePullPolicy(),
							Resources:       node.Resources,
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
							StartupProbe:   &probe,
							LivenessProbe:  &probe,
							ReadinessProbe: &readinessProbe,
							VolumeMounts:   volumeMounts,
						},
					},
					InitContainers: []corev1.Container{{
						Name:    "init",
						Image:   "public.ecr.aws/opsterio/busybox:latest",
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
					ImagePullSecrets:   image.ImagePullSecrets,
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

	// Append additional config to env vars
	keys := make([]string, 0, len(extraConfig))
	for key := range extraConfig {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, k := range keys {
		sts.Spec.Template.Spec.Containers[0].Env = append(sts.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  k,
			Value: extraConfig[k],
		})
	}
	// Append additional env vars from cr.Spec.NodePool.env
	sts.Spec.Template.Spec.Containers[0].Env = append(sts.Spec.Template.Spec.Containers[0].Env, node.Env...)

	if cr.Spec.General.SetVMMaxMapCount {
		sts.Spec.Template.Spec.InitContainers = append(sts.Spec.Template.Spec.InitContainers, corev1.Container{
			Name:  "init-sysctl",
			Image: "public.ecr.aws/opsterio/busybox:1.27.2",
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

func NewDiscoveryServiceForCR(cr *opsterv1.OpenSearchCluster) *corev1.Service {
	labels := map[string]string{
		ClusterLabel: cr.Name,
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DiscoveryServiceName(cr),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			PublishNotReadyAddresses: true,
			Ports: []corev1.ServicePort{
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
			ClusterIP: corev1.ClusterIPNone,
			Selector:  labels,
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

func NewBootstrapPod(
	cr *opsterv1.OpenSearchCluster,
	volumes []corev1.Volume,
	volumeMounts []corev1.VolumeMount,
) *corev1.Pod {
	labels := map[string]string{
		ClusterLabel: cr.Name,
	}
	resources := cr.Spec.Bootstrap.Resources

	var jvm string
	if cr.Spec.Bootstrap.Jvm == "" {
		jvm = "-Xmx512M -Xms512M"
	} else {
		jvm = cr.Spec.Bootstrap.Jvm
	}

	image := helpers.ResolveImage(cr, nil)

	probe := corev1.Probe{
		PeriodSeconds:       20,
		TimeoutSeconds:      5,
		FailureThreshold:    10,
		SuccessThreshold:    1,
		InitialDelaySeconds: 10,
		ProbeHandler:        corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.IntOrString{IntVal: cr.Spec.General.HttpPort}}},
	}

	volumes = append(volumes, corev1.Volume{
		Name: "data",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "data",
		MountPath: "/usr/share/opensearch/data",
	})

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BootstrapPodName(cr),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Env: []corev1.EnvVar{
						{
							Name:  "cluster.initial_master_nodes",
							Value: BootstrapPodName(cr),
						},
						{
							Name:  "discovery.seed_hosts",
							Value: DiscoveryServiceName(cr),
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
							Value: "master",
						},
						{
							Name:  "http.port",
							Value: fmt.Sprint(cr.Spec.General.HttpPort),
						},
					},

					Name:            "opensearch",
					Image:           image.GetImage(),
					ImagePullPolicy: image.GetImagePullPolicy(),
					Resources:       resources,
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
			InitContainers: []corev1.Container{
				{
					Name:    "init",
					Image:   "public.ecr.aws/opsterio/busybox:1.27.2",
					Command: []string{"sh", "-c"},
					Args:    []string{"chown -R 1000:1000 /usr/share/opensearch/data"},
					SecurityContext: &corev1.SecurityContext{
						RunAsUser: pointer.Int64(0),
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
			NodeSelector:       cr.Spec.Bootstrap.NodeSelector,
			Tolerations:        cr.Spec.Bootstrap.Tolerations,
			Affinity:           cr.Spec.Bootstrap.Affinity,
			ImagePullSecrets:   image.ImagePullSecrets,
		},
	}

	if cr.Spec.General.SetVMMaxMapCount {
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:  "init-sysctl",
			Image: "public.ecr.aws/opsterio/busybox:1.27.2",
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

	return pod
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

func PasswordSecret(cr *opsterv1.OpenSearchCluster, password string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-admin-password", cr.Name),
			Namespace: cr.Namespace,
		},
		StringData: map[string]string{
			"password": password,
		},
	}
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

func DiscoveryServiceName(cr *opsterv1.OpenSearchCluster) string {
	return fmt.Sprintf("%s-discovery", cr.Name)
}

func BootstrapPodName(cr *opsterv1.OpenSearchCluster) string {
	return fmt.Sprintf("%s-bootstrap-0", cr.Name)
}

func WorkingPodForRollingRestart(sts *appsv1.StatefulSet) string {
	ordinal := pointer.Int32Deref(sts.Spec.Replicas, 1) - 1 - sts.Status.UpdatedReplicas
	return ReplicaHostName(*sts, ordinal)
}

func STSInNodePools(sts appsv1.StatefulSet, nodepools []opsterv1.NodePool) bool {
	for _, nodepool := range nodepools {
		if sts.Labels[NodePoolLabel] == nodepool.Component {
			return true
		}
	}
	return false
}

func NewSecurityconfigUpdateJob(
	instance *opsterv1.OpenSearchCluster,
	jobName string,
	namespace string,
	checksum string,
	adminCertName string,
	clusterName string,
	volumes []corev1.Volume,
	volumeMounts []corev1.VolumeMount,
) batchv1.Job {
	dns := DnsOfService(instance)
	adminCert := "/certs/tls.crt"
	adminKey := "/certs/tls.key"
	caCert := "/certs/ca.crt"

	// Dummy node spec required to resolve image
	node := opsterv1.NodePool{
		Component: "securityconfig",
	}

	volumes = append(volumes, corev1.Volume{
		Name:         "admin-cert",
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: adminCertName}},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "admin-cert",
		MountPath: "/certs",
	})
	//Following httpPort, securityconfigPath are used for executing securityadmin.sh
	httpPort, securityconfigPath := helpers.VersionCheck(instance)
	// The following curl command is added to make sure cluster is full connected before .opendistro_security is created.
	arg := "ADMIN=/usr/share/opensearch/plugins/opensearch-security/tools/securityadmin.sh;" +
		"chmod +x $ADMIN;" +
		fmt.Sprintf("until curl -k --silent https://%s.svc.cluster.local:%v; do", dns, instance.Spec.General.HttpPort) +
		" echo 'Waiting to connect to the cluster'; sleep 120; " +
		"done; " +
		"count=0;" +
		fmt.Sprintf("until $ADMIN -cacert %s -cert %s -key %s -cd %s -icl -nhnv -h %s.svc.cluster.local -p %v || (( count++ >= 20 )); do", caCert, adminCert, adminKey, securityconfigPath, dns, httpPort) +
		"  sleep 20; " +
		"done"
	annotations := map[string]string{
		securityconfigChecksumAnnotation: checksum,
	}
	terminationGracePeriodSeconds := int64(5)
	backoffLimit := int32(0)

	image := helpers.ResolveImage(instance, &node)

	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: namespace, Annotations: annotations},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Name: jobName},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{{
						Name:            "updater",
						Image:           image.GetImage(),
						ImagePullPolicy: image.GetImagePullPolicy(),
						Command:         []string{"/bin/bash", "-c"},
						Args:            []string{arg},
						VolumeMounts:    volumeMounts,
					}},
					Volumes:          volumes,
					RestartPolicy:    corev1.RestartPolicyNever,
					ImagePullSecrets: image.ImagePullSecrets,
				},
			},
		},
	}
}

func AllMastersReady(ctx context.Context, k8sClient client.Client, cr *opsterv1.OpenSearchCluster) bool {
	for _, nodePool := range cr.Spec.NodePools {
		if helpers.ContainsString(nodePool.Roles, "master") {
			sts := &appsv1.StatefulSet{}
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      StsName(cr, &nodePool),
				Namespace: cr.Namespace,
			}, sts); err != nil {
				return false
			}
			if sts.Status.ReadyReplicas != pointer.Int32Deref(sts.Spec.Replicas, 1) {
				return false
			}
		}
	}
	return true
}

func DataNodesCount(ctx context.Context, k8sClient client.Client, cr *opsterv1.OpenSearchCluster) int32 {
	count := int32(0)
	for _, nodePool := range cr.Spec.NodePools {
		if helpers.ContainsString(nodePool.Roles, "data") {
			sts := &appsv1.StatefulSet{}
			if err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      StsName(cr, &nodePool),
				Namespace: cr.Namespace,
			}, sts); err == nil {
				count = count + pointer.Int32Deref(sts.Spec.Replicas, 1)
			}
		}
	}
	return count
}
