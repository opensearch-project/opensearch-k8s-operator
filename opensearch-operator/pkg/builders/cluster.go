package builders

import (
	"context"
	"fmt"
	"strings"

	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/// package that declare and build all the resources that related to the OpenSearch cluster ///

const (
	ConfigurationChecksumAnnotation  = "opster.io/config"
	DefaultDiskSize                  = "30Gi"
	defaultMonitoringPlugin          = "https://github.com/aiven/prometheus-exporter-plugin-for-opensearch/releases/download/%s.0/prometheus-exporter-%s.0.zip"
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
	// To make sure disksize is not passed as empty
	var disksize string
	if len(node.DiskSize) == 0 {
		disksize = DefaultDiskSize
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
		"search",
	}
	var selectedRoles []string
	for _, role := range node.Roles {
		if helpers.ContainsString(availableRoles, role) {
			role = helpers.MapClusterRole(role, cr.Spec.General.Version)
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
					if node.Persistence.PVC.StorageClassName == "" {
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

	labels := map[string]string{
		helpers.ClusterLabel:  cr.Name,
		helpers.NodePoolLabel: node.Component,
	}
	annotations := map[string]string{
		ConfigurationChecksumAnnotation: configChecksum,
	}
	matchLabels := map[string]string{
		helpers.ClusterLabel:  cr.Name,
		helpers.NodePoolLabel: node.Component,
	}

	if helpers.ContainsString(selectedRoles, "master") {
		labels["opensearch.role"] = "master"
	}

	if helpers.ContainsString(selectedRoles, "cluster_manager") {
		labels["opensearch.role"] = "cluster_manager"
	}

	// cr.Spec.NodePool.labels
	for k, v := range node.Labels {
		labels[k] = v
	}

	// cr.Spec.NodePool.annotations
	for ak, vk := range node.Annotations {
		annotations[ak] = vk
	}

	runas := int64(0)

	if cr.Spec.General.Vendor == "Op" || cr.Spec.General.Vendor == "OP" ||
		cr.Spec.General.Vendor == "Opensearch" ||
		cr.Spec.General.Vendor == "opensearch" ||
		cr.Spec.General.Vendor == "" {
		//	vendor = "opensearchproject/opensearch"
	} else {
		panic("vendor=elasticsearch not implemented")
		// vendor ="elasticsearch"
	}

	jvm := helpers.CalculateJvmHeapSize(&node)

	// If node role `search` defined add required experimental flag if version less than 2.7
	if helpers.ContainsString(selectedRoles, "search") && helpers.CompareVersions(cr.Spec.General.Version, "2.7.0") {
		jvm += " -Dopensearch.experimental.feature.searchable_snapshot.enabled=true"
	}

	// Supress repeated log messages about a deprecated format for the publish address
	jvm += " -Dopensearch.transport.cname_in_publish_address=true"

	startupProbePeriodSeconds := int32(20)
	startupProbeTimeoutSeconds := int32(5)
	startupProbeFailureThreshold := int32(10)
	startupProbeSuccessThreshold := int32(1)
	startupProbeInitialDelaySeconds := int32(10)

	readinessProbePeriodSeconds := int32(30)
	readinessProbeTimeoutSeconds := int32(30)
	readinessProbeFailureThreshold := int32(5)
	readinessProbeInitialDelaySeconds := int32(60)

	livenessProbePeriodSeconds := int32(20)
	livenessProbeTimeoutSeconds := int32(5)
	livenessProbeFailureThreshold := int32(10)
	livenessProbeSuccessThreshold := int32(1)
	livenessProbeInitialDelaySeconds := int32(10)

	if node.Probes != nil {
		if node.Probes.Liveness != nil {
			if node.Probes.Liveness.InitialDelaySeconds > 0 {
				livenessProbeInitialDelaySeconds = node.Probes.Liveness.InitialDelaySeconds
			}

			if node.Probes.Liveness.PeriodSeconds > 0 {
				livenessProbePeriodSeconds = node.Probes.Liveness.PeriodSeconds
			}

			if node.Probes.Liveness.TimeoutSeconds > 0 {
				livenessProbeTimeoutSeconds = node.Probes.Liveness.TimeoutSeconds
			}

			if node.Probes.Liveness.FailureThreshold > 0 {
				livenessProbeFailureThreshold = node.Probes.Liveness.FailureThreshold
			}

			if node.Probes.Liveness.SuccessThreshold > 0 {
				livenessProbeSuccessThreshold = node.Probes.Liveness.SuccessThreshold
			}
		}

		if node.Probes.Startup != nil {
			if node.Probes.Startup.InitialDelaySeconds > 0 {
				startupProbeInitialDelaySeconds = node.Probes.Startup.InitialDelaySeconds
			}

			if node.Probes.Startup.PeriodSeconds > 0 {
				startupProbePeriodSeconds = node.Probes.Startup.PeriodSeconds
			}

			if node.Probes.Startup.TimeoutSeconds > 0 {
				startupProbeTimeoutSeconds = node.Probes.Startup.TimeoutSeconds
			}

			if node.Probes.Startup.FailureThreshold > 0 {
				startupProbeFailureThreshold = node.Probes.Startup.FailureThreshold
			}

			if node.Probes.Startup.SuccessThreshold > 0 {
				startupProbeSuccessThreshold = node.Probes.Startup.SuccessThreshold
			}
		}

		if node.Probes.Readiness != nil {
			if node.Probes.Readiness.InitialDelaySeconds > 0 {
				readinessProbeInitialDelaySeconds = node.Probes.Readiness.InitialDelaySeconds
			}

			if node.Probes.Readiness.PeriodSeconds > 0 {
				readinessProbePeriodSeconds = node.Probes.Readiness.PeriodSeconds
			}

			if node.Probes.Readiness.TimeoutSeconds > 0 {
				readinessProbeTimeoutSeconds = node.Probes.Readiness.TimeoutSeconds
			}

			if node.Probes.Readiness.FailureThreshold > 0 {
				readinessProbeFailureThreshold = node.Probes.Readiness.FailureThreshold
			}
		}
	}

	livenessProbe := corev1.Probe{
		PeriodSeconds:       livenessProbePeriodSeconds,
		TimeoutSeconds:      livenessProbeTimeoutSeconds,
		FailureThreshold:    livenessProbeFailureThreshold,
		SuccessThreshold:    livenessProbeSuccessThreshold,
		InitialDelaySeconds: livenessProbeInitialDelaySeconds,
		ProbeHandler:        corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.IntOrString{IntVal: cr.Spec.General.HttpPort}}},
	}

	startupProbe := corev1.Probe{
		PeriodSeconds:       startupProbePeriodSeconds,
		TimeoutSeconds:      startupProbeTimeoutSeconds,
		FailureThreshold:    startupProbeFailureThreshold,
		SuccessThreshold:    startupProbeSuccessThreshold,
		InitialDelaySeconds: startupProbeInitialDelaySeconds,
		ProbeHandler:        corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.IntOrString{IntVal: cr.Spec.General.HttpPort}}},
	}

	// Because the http endpoint requires auth we need to do it as a curl script
	httpPort := PortForCluster(cr)

	curlCmd := "curl -k -u \"$(cat /mnt/admin-credentials/username):$(cat /mnt/admin-credentials/password)\" --silent --fail https://localhost:" + fmt.Sprint(httpPort)
	readinessProbe := corev1.Probe{
		InitialDelaySeconds: readinessProbeInitialDelaySeconds,
		PeriodSeconds:       readinessProbePeriodSeconds,
		FailureThreshold:    readinessProbeFailureThreshold,
		TimeoutSeconds:      readinessProbeTimeoutSeconds,
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

	volumes = append(volumes, corev1.Volume{
		Name: "admin-credentials",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: fmt.Sprintf("%s-admin-password", cr.Name)},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "admin-credentials",
		MountPath: "/mnt/admin-credentials",
	})

	image := helpers.ResolveImage(cr, &node)
	initHelperImage := helpers.ResolveInitHelperImage(cr)
	resources := cr.Spec.InitHelper.Resources

	startUpCommand := "./opensearch-docker-entrypoint.sh"
	// If a custom command is specified, use it.
	if len(cr.Spec.General.Command) > 0 {
		startUpCommand = cr.Spec.General.Command
	}

	var pluginslist []string
	if cr.Spec.General.Monitoring.Enable {
		if cr.Spec.General.Monitoring.PluginURL != "" {
			pluginslist = append(pluginslist, cr.Spec.General.Monitoring.PluginURL)
		} else {
			pluginslist = append(pluginslist, fmt.Sprintf(defaultMonitoringPlugin, cr.Spec.General.Version, cr.Spec.General.Version))
		}
	}

	pluginslist = helpers.RemoveDuplicateStrings(append(pluginslist, cr.Spec.General.PluginsList...))

	mainCommand := helpers.BuildMainCommand("./bin/opensearch-plugin", pluginslist, true, startUpCommand)

	podSecurityContext := cr.Spec.General.PodSecurityContext
	securityContext := cr.Spec.General.SecurityContext

	var initContainers []corev1.Container
	if !helpers.SkipInitContainer() {
		initContainers = append(initContainers, corev1.Container{
			Name:            "init",
			Image:           initHelperImage.GetImage(),
			ImagePullPolicy: initHelperImage.GetImagePullPolicy(),
			Resources:       resources,
			Command:         []string{"sh", "-c"},
			Args:            []string{"chown -R 1000:1000 /usr/share/opensearch/data"},
			SecurityContext: &corev1.SecurityContext{
				RunAsUser: &runas,
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "data",
					MountPath: "/usr/share/opensearch/data",
				},
			},
		})
	}

	// If Keystore Values are set in OpenSearchCluster manifest
	if cr.Spec.General.Keystore != nil && len(cr.Spec.General.Keystore) > 0 {

		// Add volume and volume mount for keystore
		volumes = append(volumes, corev1.Volume{
			Name: "keystore",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "keystore",
			MountPath: "/usr/share/opensearch/config/opensearch.keystore",
			SubPath:   "opensearch.keystore",
		})

		initContainerVolumeMounts := []corev1.VolumeMount{
			{
				Name:      "keystore",
				MountPath: "/tmp/keystore",
			},
		}

		// Add volumes and volume mounts for keystore secrets
		for _, keystoreValue := range cr.Spec.General.Keystore {
			volumes = append(volumes, corev1.Volume{
				Name: "keystore-" + keystoreValue.Secret.Name,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: keystoreValue.Secret.Name,
					},
				},
			})

			if keystoreValue.KeyMappings == nil || len(keystoreValue.KeyMappings) == 0 {
				// If no renames are necessary, mount secret key-value pairs directly
				initContainerVolumeMounts = append(initContainerVolumeMounts, corev1.VolumeMount{
					Name:      "keystore-" + keystoreValue.Secret.Name,
					MountPath: "/tmp/keystoreSecrets/" + keystoreValue.Secret.Name,
				})
			} else {
				keys := helpers.SortedKeys(keystoreValue.KeyMappings)
				for _, oldKey := range keys {
					initContainerVolumeMounts = append(initContainerVolumeMounts, corev1.VolumeMount{
						Name:      "keystore-" + keystoreValue.Secret.Name,
						MountPath: "/tmp/keystoreSecrets/" + keystoreValue.Secret.Name + "/" + keystoreValue.KeyMappings[oldKey],
						SubPath:   oldKey,
					})
				}
			}
		}

		keystoreInitContainer := corev1.Container{
			Name:            "keystore",
			Image:           image.GetImage(),
			ImagePullPolicy: image.GetImagePullPolicy(),
			Resources:       resources,
			Command: []string{
				"sh",
				"-c",
				`
				#!/usr/bin/env bash
				set -euo pipefail

				if [ ! -f /usr/share/opensearch/config/opensearch.keystore ]; then
				  /usr/share/opensearch/bin/opensearch-keystore create
				fi
				for i in /tmp/keystoreSecrets/*/*; do
				  key=$(basename $i)
				  echo "Adding file $i to keystore key $key"
				  /usr/share/opensearch/bin/opensearch-keystore add-file "$key" "$i" --force
				done

				# Add the bootstrap password since otherwise the opensearch entrypoint tries to do this on startup
				if [ ! -z ${PASSWORD+x} ]; then
				  echo 'Adding env $PASSWORD to keystore as key bootstrap.password'
				  echo "$PASSWORD" | /usr/share/opensearch/bin/opensearch-keystore add -x bootstrap.password
				fi

				cp -a /usr/share/opensearch/config/opensearch.keystore /tmp/keystore/
				`,
			},
			VolumeMounts:    initContainerVolumeMounts,
			SecurityContext: securityContext,
		}

		initContainers = append(initContainers, keystoreInitContainer)
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.Name + "-" + node.Component,
			Namespace:   cr.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &node.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			PodManagementPolicy: appsv1.OrderedReadyPodManagement,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.OnDeleteStatefulSetStrategyType,
			},
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
							StartupProbe:    &startupProbe,
							LivenessProbe:   &livenessProbe,
							ReadinessProbe:  &readinessProbe,
							VolumeMounts:    volumeMounts,
							SecurityContext: securityContext,
						},
					},
					InitContainers:            initContainers,
					Volumes:                   volumes,
					ServiceAccountName:        cr.Spec.General.ServiceAccount,
					NodeSelector:              node.NodeSelector,
					Tolerations:               node.Tolerations,
					Affinity:                  node.Affinity,
					TopologySpreadConstraints: node.TopologySpreadConstraints,
					ImagePullSecrets:          image.ImagePullSecrets,
					PriorityClassName:         node.PriorityClassName,
					SecurityContext:           podSecurityContext,
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
	keys := helpers.SortedKeys(extraConfig)
	for _, k := range keys {
		sts.Spec.Template.Spec.Containers[0].Env = append(sts.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  k,
			Value: extraConfig[k],
		})
	}
	// Append additional env vars from cr.Spec.NodePool.env
	sts.Spec.Template.Spec.Containers[0].Env = append(sts.Spec.Template.Spec.Containers[0].Env, node.Env...)

	if cr.Spec.General.SetVMMaxMapCount {
		initHelperImage := helpers.ResolveInitHelperImage(cr)

		sts.Spec.Template.Spec.InitContainers = append(sts.Spec.Template.Spec.InitContainers, corev1.Container{
			Name:            "init-sysctl",
			Image:           initHelperImage.GetImage(),
			ImagePullPolicy: initHelperImage.GetImagePullPolicy(),
			Resources:       resources,
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
		helpers.ClusterLabel:  cr.Name,
		helpers.NodePoolLabel: nodePool.Component,
	}

	annotations := make(map[string]string)

	for key, value := range cr.Spec.General.Annotations {
		annotations[key] = value
	}

	for key, value := range nodePool.Annotations {
		annotations[key] = value
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s", cr.Spec.General.ServiceName, nodePool.Component),
			Namespace:   cr.Namespace,
			Labels:      labels,
			Annotations: annotations,
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
		helpers.ClusterLabel: cr.Name,
	}
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.Spec.General.ServiceName,
			Namespace:   cr.Namespace,
			Labels:      labels,
			Annotations: cr.Spec.General.Annotations,
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
		helpers.ClusterLabel: cr.Name,
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
		helpers.ClusterLabel: cr.Name,
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
		helpers.ClusterLabel: cr.Name,
	}
	resources := cr.Spec.Bootstrap.Resources

	var jvm string
	if cr.Spec.Bootstrap.Jvm == "" {
		jvm = "-Xmx512M -Xms512M"
	} else {
		jvm = cr.Spec.Bootstrap.Jvm
	}

	image := helpers.ResolveImage(cr, nil)
	initHelperImage := helpers.ResolveInitHelperImage(cr)
	masterRole := helpers.ResolveClusterManagerRole(cr.Spec.General.Version)

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

	podSecurityContext := cr.Spec.General.PodSecurityContext
	securityContext := cr.Spec.General.SecurityContext

	env := []corev1.EnvVar{
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
			Value: masterRole,
		},
		{
			Name:  "http.port",
			Value: fmt.Sprint(cr.Spec.General.HttpPort),
		},
	}

	// Append additional config to env vars, use General.AdditionalConfig by default, overwrite with Bootstrap.AdditionalConfig
	extraConfig := cr.Spec.General.AdditionalConfig
	if cr.Spec.Bootstrap.AdditionalConfig != nil {
		extraConfig = cr.Spec.Bootstrap.AdditionalConfig
	}

	keys := helpers.SortedKeys(extraConfig)
	for _, k := range keys {
		env = append(env, corev1.EnvVar{
			Name:  k,
			Value: extraConfig[k],
		})
	}

	if cr.Spec.Bootstrap.Env != nil {
		env = append(env, cr.Spec.Bootstrap.Env...)
	}

	var initContainers []corev1.Container
	if !helpers.SkipInitContainer() {
		initContainers = append(initContainers, corev1.Container{
			Name:            "init",
			Image:           initHelperImage.GetImage(),
			ImagePullPolicy: initHelperImage.GetImagePullPolicy(),
			Resources:       cr.Spec.InitHelper.Resources,
			Command:         []string{"sh", "-c"},
			Args:            []string{"chown -R 1000:1000 /usr/share/opensearch/data"},
			SecurityContext: &corev1.SecurityContext{
				RunAsUser: pointer.Int64(0),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "data",
					MountPath: "/usr/share/opensearch/data",
				},
			},
		})
	}

	// If Keystore Values are set in OpenSearchCluster manifest
	if cr.Spec.Bootstrap.Keystore != nil && len(cr.Spec.Bootstrap.Keystore) > 0 {

		// Add volume and volume mount for keystore
		volumes = append(volumes, corev1.Volume{
			Name: "keystore",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "keystore",
			MountPath: "/usr/share/opensearch/config/opensearch.keystore",
			SubPath:   "opensearch.keystore",
		})

		initContainerVolumeMounts := []corev1.VolumeMount{
			{
				Name:      "keystore",
				MountPath: "/tmp/keystore",
			},
		}

		// Add volumes and volume mounts for keystore secrets
		for _, keystoreValue := range cr.Spec.Bootstrap.Keystore {
			volumes = append(volumes, corev1.Volume{
				Name: "keystore-" + keystoreValue.Secret.Name,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: keystoreValue.Secret.Name,
					},
				},
			})

			if keystoreValue.KeyMappings == nil || len(keystoreValue.KeyMappings) == 0 {
				// If no renames are necessary, mount secret key-value pairs directly
				initContainerVolumeMounts = append(initContainerVolumeMounts, corev1.VolumeMount{
					Name:      "keystore-" + keystoreValue.Secret.Name,
					MountPath: "/tmp/keystoreSecrets/" + keystoreValue.Secret.Name,
				})
			} else {
				keys := helpers.SortedKeys(keystoreValue.KeyMappings)
				for _, oldKey := range keys {
					initContainerVolumeMounts = append(initContainerVolumeMounts, corev1.VolumeMount{
						Name:      "keystore-" + keystoreValue.Secret.Name,
						MountPath: "/tmp/keystoreSecrets/" + keystoreValue.Secret.Name + "/" + keystoreValue.KeyMappings[oldKey],
						SubPath:   oldKey,
					})
				}
			}
		}

		keystoreInitContainer := corev1.Container{
			Name:            "keystore",
			Image:           image.GetImage(),
			ImagePullPolicy: image.GetImagePullPolicy(),
			Resources:       resources,
			Command: []string{
				"sh",
				"-c",
				`
				#!/usr/bin/env bash
				set -euo pipefail

				if [ ! -f /usr/share/opensearch/config/opensearch.keystore ]; then
				  /usr/share/opensearch/bin/opensearch-keystore create
				fi
				for i in /tmp/keystoreSecrets/*/*; do
				  key=$(basename $i)
				  echo "Adding file $i to keystore key $key"
				  /usr/share/opensearch/bin/opensearch-keystore add-file "$key" "$i" --force
				done

				# Add the bootstrap password since otherwise the opensearch entrypoint tries to do this on startup
				if [ ! -z ${PASSWORD+x} ]; then
				  echo 'Adding env $PASSWORD to keystore as key bootstrap.password'
				  echo "$PASSWORD" | /usr/share/opensearch/bin/opensearch-keystore add -x bootstrap.password
				fi

				cp -a /usr/share/opensearch/config/opensearch.keystore /tmp/keystore/
				`,
			},
			VolumeMounts:    initContainerVolumeMounts,
			SecurityContext: securityContext,
		}

		initContainers = append(initContainers, keystoreInitContainer)
	}

	startUpCommand := "./opensearch-docker-entrypoint.sh"

	pluginslist := helpers.RemoveDuplicateStrings(cr.Spec.Bootstrap.PluginsList)
	mainCommand := helpers.BuildMainCommand("./bin/opensearch-plugin", pluginslist, true, startUpCommand)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BootstrapPodName(cr),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Env:             env,
					Name:            "opensearch",
					Command:         mainCommand,
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
					StartupProbe:    &probe,
					LivenessProbe:   &probe,
					VolumeMounts:    volumeMounts,
					SecurityContext: securityContext,
				},
			},
			InitContainers:     initContainers,
			Volumes:            volumes,
			ServiceAccountName: cr.Spec.General.ServiceAccount,
			NodeSelector:       cr.Spec.Bootstrap.NodeSelector,
			Tolerations:        cr.Spec.Bootstrap.Tolerations,
			Affinity:           cr.Spec.Bootstrap.Affinity,
			ImagePullSecrets:   image.ImagePullSecrets,
			SecurityContext:    podSecurityContext,
		},
	}

	if cr.Spec.General.SetVMMaxMapCount {
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:            "init-sysctl",
			Image:           initHelperImage.GetImage(),
			ImagePullPolicy: initHelperImage.GetImagePullPolicy(),
			Resources:       cr.Spec.InitHelper.Resources,
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
	return fmt.Sprintf("https://%s.svc.%s:%d", DnsOfService(cr), helpers.ClusterDnsBase(), httpPort)
}

func PasswordSecret(cr *opsterv1.OpenSearchCluster, username, password string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-admin-password", cr.Name),
			Namespace: cr.Namespace,
		},
		StringData: map[string]string{
			"username": username,
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

func DiscoveryServiceName(cr *opsterv1.OpenSearchCluster) string {
	return fmt.Sprintf("%s-discovery", cr.Name)
}

func BootstrapPodName(cr *opsterv1.OpenSearchCluster) string {
	return fmt.Sprintf("%s-bootstrap-0", cr.Name)
}

func STSInNodePools(sts appsv1.StatefulSet, nodepools []opsterv1.NodePool) bool {
	for _, nodepool := range nodepools {
		if sts.Labels[helpers.NodePoolLabel] == nodepool.Component {
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
	cmdArg string,
	volumes []corev1.Volume,
	volumeMounts []corev1.VolumeMount,
) batchv1.Job {
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

	annotations := map[string]string{
		securityconfigChecksumAnnotation: checksum,
	}
	terminationGracePeriodSeconds := int64(5)
	backoffLimit := int32(0)

	image := helpers.ResolveImage(instance, &node)
	securityContext := instance.Spec.General.SecurityContext
	podSecurityContext := instance.Spec.General.PodSecurityContext
	resources := instance.Spec.Security.GetConfig().GetUpdateJob().Resources
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
						Resources:       resources,
						Command:         []string{"/bin/bash", "-c"},
						Args:            []string{cmdArg},
						VolumeMounts:    volumeMounts,
						SecurityContext: securityContext,
					}},
					ServiceAccountName: instance.Spec.General.ServiceAccount,
					Volumes:            volumes,
					RestartPolicy:      corev1.RestartPolicyNever,
					ImagePullSecrets:   image.ImagePullSecrets,
					SecurityContext:    podSecurityContext,
				},
			},
		},
	}
}

func AllMastersReady(ctx context.Context, k8sClient client.Client, cr *opsterv1.OpenSearchCluster) bool {
	for _, nodePool := range cr.Spec.NodePools {
		masterRole := helpers.ResolveClusterManagerRole(cr.Spec.General.Version)
		if helpers.ContainsString(helpers.MapClusterRoles(nodePool.Roles, cr.Spec.General.Version), masterRole) {
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

func NewServiceMonitor(cr *opsterv1.OpenSearchCluster) *monitoring.ServiceMonitor {
	labels := map[string]string{
		helpers.ClusterLabel: cr.Name,
	}
	selector := metav1.LabelSelector{
		MatchLabels: labels,
		// Needed so only the pool-specific service is matched, otherwise there would be double scraping
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      helpers.NodePoolLabel,
				Operator: metav1.LabelSelectorOpExists,
			},
		},
	}

	namespaceSelector := monitoring.NamespaceSelector{
		Any:        false,
		MatchNames: []string{cr.Namespace},
	}

	if cr.Spec.General.Monitoring.ScrapeInterval == "" {
		cr.Spec.General.Monitoring.ScrapeInterval = "30s"
	}
	user := monitoring.BasicAuth{}

	monitorUser := cr.Spec.General.Monitoring.MonitoringUserSecret
	var basicAuthSecret string
	if monitorUser == "" {
		basicAuthSecret = cr.Name + "-admin-password"
		// Use admin credentials if no separate monitoring user was defined
	} else {
		basicAuthSecret = cr.Spec.General.Monitoring.MonitoringUserSecret
	}

	user = monitoring.BasicAuth{
		Username: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: basicAuthSecret},
			Key:                  "username",
		},
		Password: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: basicAuthSecret},
			Key:                  "password",
		},
	}

	var tlsconfig *monitoring.TLSConfig
	if cr.Spec.General.Monitoring.TLSConfig != nil {
		safetlsconfig := monitoring.SafeTLSConfig{
			ServerName:         cr.Spec.General.Monitoring.TLSConfig.ServerName,
			InsecureSkipVerify: cr.Spec.General.Monitoring.TLSConfig.InsecureSkipVerify,
		}

		tlsconfig = &monitoring.TLSConfig{
			SafeTLSConfig: safetlsconfig,
		}
	} else {
		tlsconfig = nil
	}

	monitorLabel := map[string]string{
		helpers.ClusterLabel: cr.Name,
	}
	for k, v := range cr.Spec.General.Monitoring.Labels {
		monitorLabel[k] = v
	}

	return &monitoring.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-monitor",
			Namespace: cr.Namespace,
			Labels:    monitorLabel,
		},
		Spec: monitoring.ServiceMonitorSpec{
			JobLabel: cr.Name + "-monitor",
			TargetLabels: []string{
				helpers.ClusterLabel,
			},
			PodTargetLabels: []string{
				helpers.ClusterLabel,
			},
			Endpoints: []monitoring.Endpoint{
				{
					Port:            "http",
					TargetPort:      nil,
					Path:            "/_prometheus/metrics",
					Interval:        monitoring.Duration(cr.Spec.General.Monitoring.ScrapeInterval),
					TLSConfig:       tlsconfig,
					BearerTokenFile: "",
					HonorLabels:     false,
					BasicAuth:       &user,
					Scheme:          "https",
				},
			},
			Selector:          selector,
			NamespaceSelector: namespaceSelector,
		},
	}
}
