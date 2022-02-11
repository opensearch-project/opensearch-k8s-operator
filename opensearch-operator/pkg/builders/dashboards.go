package builders

import (
	"fmt"

	sts "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	opsterv1 "opensearch.opster.io/api/v1"
)

/// Package that declare and build all the resources that related to the OpenSearch-Dashboard ///

func NewDashboardsDeploymentForCR(cr *opsterv1.OpenSearchCluster) *sts.Deployment {

	labels := map[string]string{
		"opensearch.cluster.dashboards": cr.Name,
	}
	var rep int32 = 1
	var port int32 = 5601
	var mode int32 = 420

	probe := corev1.Probe{
		PeriodSeconds:       20,
		TimeoutSeconds:      5,
		FailureThreshold:    10,
		SuccessThreshold:    1,
		InitialDelaySeconds: 10,
		ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/api/status", Port: intstr.IntOrString{IntVal: port}, Scheme: "HTTP"}},
	}

	return &sts.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Spec.General.ClusterName + "-dashboards",
			Namespace: cr.Spec.General.ClusterName,
			Labels:    labels,
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
						{
							Name: "dashboards-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									DefaultMode:          &mode,
									LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-dashboards-config", cr.Spec.General.ClusterName)},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name: "dashboards",
							//	Image: "docker.elastic.co/kibana/kibana:" + cr.Spec.General.Version,
							Image: "opensearchproject/opensearch-dashboards:1.0.0",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: port,
								},
							},
							StartupProbe:  &probe,
							LivenessProbe: &probe,
							Env: []corev1.EnvVar{
								{
									Name:  "OPENSEARCH_HOSTS",
									Value: "https://" + cr.Spec.General.ServiceName + "." + cr.Spec.General.ClusterName + ".svc.cluster.local:9200",
								},
								{
									Name:  "SERVER_HOST",
									Value: "0.0.0.0",
								},
								// Temporary until securityconfig controller is implemented
								{
									Name:  "OPENSEARCH_USERNAME",
									Value: "admin",
								},
								{
									Name:  "OPENSEARCH_PASSWORD",
									Value: "admin",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "dashboards-config",
									MountPath: "/usr/share/opensearch-dashboards/config/opensearch_dashboards.yml",
									SubPath:   "opensearch_dashboards.yml",
								},
							},
						},
					},
				},
			},
		},
	}
}

func NewDashboardsConfigMapForCR(cr *opsterv1.OpenSearchCluster, name string) *corev1.ConfigMap {

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Spec.General.ClusterName,
		},
		Data: map[string]string{
			"opensearch_dashboards.yml": "server.name: " + cr.Spec.General.ClusterName + "-dashboards" + "\nopensearch.ssl.verificationMode: none\n",
		},
	}
}

func NewDashboardsSvcForCr(cr *opsterv1.OpenSearchCluster) *corev1.Service {

	var port int32 = 5601

	labels := map[string]string{
		"opensearch.cluster.dashboards": cr.Name,
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Spec.General.ServiceName + "-dashboards",
			Namespace: cr.Spec.General.ClusterName,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:     "http",
				Protocol: "TCP",
				Port:     port,
				TargetPort: intstr.IntOrString{
					IntVal: port,
				},
			},
			},
			Selector: labels,
		},
	}
}
