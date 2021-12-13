package builders

import (
	sts "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	opsterv1 "os-operator.io/api/v1"
	"strconv"
)

/// Package that declare and build all the resources that related to the OpenSearch-Dashboard ///

func NewKibanaForCR(cr *opsterv1.Os) *sts.Deployment {

	labels := map[string]string{
		"app": cr.Name,
	}
	var rep int32 = 1
	var port int32 = 5601

	i, err := strconv.ParseInt("420", 10, 32)
	if err != nil {
		panic(err)
	}
	mode := int32(i)

	return &sts.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Spec.General.ClusterName + "-kibana",
			Namespace: cr.Spec.General.ClusterName,
			Labels: labels,
		},
		Spec: sts.DeploymentSpec{
			Replicas: &rep,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: nil,
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						corev1.Volume{
							Name: "kibana",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									DefaultMode:          &mode,
									LocalObjectReference: corev1.LocalObjectReference{Name: "kibana"},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "kibana-container",
						//	Image: "docker.elastic.co/kibana/kibana:" + cr.Spec.General.Version,
							Image: "opensearchproject/opensearch-dashboards:1.0.0",
							Ports: []corev1.ContainerPort{
								{
									Name:          "k-port-5601",
									ContainerPort: port,
								},
							},
								Env: []corev1.EnvVar{corev1.EnvVar{
									Name:      "OPENSEARCH_HOSTS",
									Value:      "https://"+cr.Spec.General.ServiceName + "-svc" + "." + cr.Spec.General.ClusterName + ":9200",
									ValueFrom: nil,
								},
									corev1.EnvVar{
										Name:      "SERVER_HOST",
										Value:     "0.0.0.0",
										ValueFrom: nil,
									},
								},
									VolumeMounts: []corev1.VolumeMount{
										{Name: "kibana",
											MountPath: "/usr/share/kibana/config/kibana.yml",
											SubPath:   "kibana.yml",
										},
									},

							},
					},
				},
			},
		},
	}
}

func NewCmKibanaForCR(cr *opsterv1.Os) *corev1.ConfigMap {

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kibana",
			Namespace: cr.Spec.General.ClusterName,
		},
		Data: map[string]string{
			"kibana.yml": "    elasticsearch.hosts: https://" + cr.Spec.General.ServiceName + "-svc." + cr.Spec.General.ClusterName + "\n    server.host: \"0\"\n    server.name: " + cr.Spec.General.ClusterName + "-kibana" + "\n    server.basePath: /es-002-kibana\n",
		},
	}
}

func NewKibanaSvcForCr(cr *opsterv1.Os) *corev1.Service {

	labels := map[string]string{
		"app": cr.Name,
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Spec.General.ServiceName + "-k-svc",
			Namespace: cr.Spec.General.ClusterName,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Name:     "kibana",
					Protocol: "TCP",
					Port:     5601,
					TargetPort: intstr.IntOrString{
						IntVal: 5601,
					},
				},
			},
			Selector: labels,
		},
	}
}
