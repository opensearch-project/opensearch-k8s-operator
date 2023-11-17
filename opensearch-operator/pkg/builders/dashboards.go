package builders

import (
	"fmt"
	"sort"
	"strings"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

/// Package that declare and build all the resources that related to the OpenSearch-Dashboard ///

func NewDashboardsDeploymentForCR(cr *opsterv1.OpenSearchCluster, volumes []corev1.Volume, volumeMounts []corev1.VolumeMount, annotations map[string]string) *appsv1.Deployment {
	var replicas int32 = cr.Spec.Dashboards.Replicas
	var port int32 = 5601
	var mode int32 = 420
	resources := cr.Spec.Dashboards.Resources

	image := helpers.ResolveDashboardsImage(cr)

	volumes = append(volumes, corev1.Volume{
		Name: "dashboards-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				DefaultMode:          &mode,
				LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-dashboards-config", cr.Name)},
			},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "dashboards-config",
		MountPath: "/usr/share/opensearch-dashboards/config/opensearch_dashboards.yml",
		SubPath:   "opensearch_dashboards.yml",
	})

	podSecurityContext := cr.Spec.Dashboards.PodSecurityContext
	securityContext := cr.Spec.Dashboards.SecurityContext

	env := []corev1.EnvVar{
		{
			Name:  "OPENSEARCH_HOSTS",
			Value: URLForCluster(cr),
		},
		{
			Name:  "SERVER_HOST",
			Value: "0.0.0.0",
		},
	}

	if len(cr.Spec.Dashboards.Env) != 0 {
		env = append(env, cr.Spec.Dashboards.Env...)
	}

	if cr.Spec.Dashboards.OpensearchCredentialsSecret.Name != "" {
		// Custom credentials supplied
		env = append(env, corev1.EnvVar{Name: "OPENSEARCH_USERNAME", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: cr.Spec.Dashboards.OpensearchCredentialsSecret, Key: "username"}}})
		env = append(env, corev1.EnvVar{Name: "OPENSEARCH_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: cr.Spec.Dashboards.OpensearchCredentialsSecret, Key: "password"}}})
	} else {
		// Default values from demo configuration
		env = append(env, corev1.EnvVar{Name: "OPENSEARCH_USERNAME", Value: "admin"})
		env = append(env, corev1.EnvVar{Name: "OPENSEARCH_PASSWORD", Value: "admin"})
	}

	labels := map[string]string{
		"opensearch.cluster.dashboards": cr.Name,
	}

	// cr.Spec.Dashboards.labels
	for key, value := range cr.Spec.Dashboards.Labels {
		labels[key] = value
	}

	// If annotations are not provided as a func parameter, such as during automated testing
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// cr.Spec.Dashboards.annotations
	for annotationsKey, annotationsVal := range cr.Spec.Dashboards.Annotations {
		annotations[annotationsKey] = annotationsVal
	}

	var probeScheme corev1.URIScheme = "HTTP"
	if cr.Spec.Dashboards.Tls != nil && cr.Spec.Dashboards.Tls.Enable {
		probeScheme = "HTTPS"
	}

	healthcheckPath := "/api/reporting/stats"

	// Figure out if the opensearch cluster runs with a base path configured
	if cr.Spec.Dashboards.BasePath != "" {
		// If basePath is correctly setup, prefix healthcheck path
		healthcheckPath = fmt.Sprintf("%s%s", cr.Spec.Dashboards.BasePath, healthcheckPath)
	}

	probe := corev1.Probe{
		PeriodSeconds:       20,
		TimeoutSeconds:      5,
		FailureThreshold:    10,
		SuccessThreshold:    1,
		InitialDelaySeconds: 10,

		/// changed from /api/status to /api/reporting/stats
		// to use /api/status add
		/*httpHeaders:
		  - name: Authorization
		    value: Basic YWRtaW46YWRtaW4=*/

		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   healthcheckPath,
				Port:   intstr.IntOrString{IntVal: port},
				Scheme: probeScheme,
			},
		},
	}

	mainCommand := helpers.BuildMainCommandOSD("./bin/opensearch-dashboards-plugin", cr.Spec.Dashboards.PluginsList, "./opensearch-dashboards-docker-entrypoint.sh")

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-dashboards",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					Volumes: volumes,
					Containers: []corev1.Container{
						{
							Name:            "dashboards",
							Image:           image.GetImage(),
							ImagePullPolicy: image.GetImagePullPolicy(),
							Resources:       resources,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: port,
								},
							},
							StartupProbe:    &probe,
							LivenessProbe:   &probe,
							Env:             env,
							VolumeMounts:    volumeMounts,
							Command:         mainCommand,
							SecurityContext: securityContext,
						},
					},
					ServiceAccountName: cr.Spec.General.ServiceAccount,
					ImagePullSecrets:   image.ImagePullSecrets,
					NodeSelector:       cr.Spec.Dashboards.NodeSelector,
					Tolerations:        cr.Spec.Dashboards.Tolerations,
					Affinity:           cr.Spec.Dashboards.Affinity,
					SecurityContext:    podSecurityContext,
				},
			},
		},
	}
}

func NewDashboardsConfigMapForCR(cr *opsterv1.OpenSearchCluster, name string, config map[string]string) *corev1.ConfigMap {
	config["server.name"] = cr.Name + "-dashboards"
	config["opensearch.ssl.verificationMode"] = "none"

	if cr.Spec.Dashboards.BasePath != "" {
		config["server.basePath"] = cr.Spec.Dashboards.BasePath
		config["server.rewriteBasePath"] = "true"
	}

	keys := make([]string, 0, len(config))

	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, key := range keys {
		sb.WriteString(fmt.Sprintf("%s: %s\n", key, config[key]))
	}
	data := sb.String()

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
		},
		Data: map[string]string{
			helpers.DashboardConfigName: data,
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
			Name:        cr.Spec.General.ServiceName + "-dashboards",
			Namespace:   cr.Namespace,
			Labels:      labels,
			Annotations: cr.Spec.Dashboards.Annotations,
		},
		Spec: corev1.ServiceSpec{
			// Type does not need to be checked for a nil value as ClusterIP is set as the default in the CRD
			Type:                     cr.Spec.Dashboards.Service.Type,
			LoadBalancerSourceRanges: cr.Spec.Dashboards.Service.LoadBalancerSourceRanges,
			Ports: []corev1.ServicePort{
				{
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
