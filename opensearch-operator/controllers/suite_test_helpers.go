package controllers

import (
	"context"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateNamespace(k8sClient client.Client, cluster *opsterv1.OpenSearchCluster) error {
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: cluster.Name}}
	return k8sClient.Create(context.Background(), &ns)
}

func IsCreated(ctx context.Context, k8sClient client.Client, obj client.Object) bool {
	if err := k8sClient.Get(ctx, client.ObjectKey{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}, obj); err != nil {
		return false
	}
	return true
}

func IsNsDeleted(k8sClient client.Client, namespace corev1.Namespace) bool {
	ns := corev1.Namespace{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: namespace.Name}, &ns); err == nil {
		return ns.Status.Phase == "Terminating"
	}
	return true
}

func IsNsCreated(k8sClient client.Client, name string) bool {
	ns := corev1.Namespace{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name}, &ns); err == nil {
		return true
	} else {
		return false
	}
}

func IsSTSDeleted(k8sClient client.Client, name string, namespace string) bool {
	sts := appsv1.StatefulSet{}
	err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name, Namespace: namespace}, &sts)
	return err != nil
}

func IsDeploymentDeleted(k8sClient client.Client, name string, namespace string) bool {
	deployment := appsv1.Deployment{}
	err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name, Namespace: namespace}, &deployment)
	return err != nil
}

func IsServiceDeleted(k8sClient client.Client, name string, namespace string) bool {
	service := corev1.Service{}
	err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name, Namespace: namespace}, &service)
	return err != nil
}

func IsSecretDeleted(k8sClient client.Client, name string, namespace string) bool {
	service := corev1.Secret{}
	err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name, Namespace: namespace}, &service)
	return err != nil
}

func IsConfigMapDeleted(k8sClient client.Client, name string, namespace string) bool {
	service := corev1.ConfigMap{}
	err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name, Namespace: namespace}, &service)
	return err != nil
}

func HasOwnerReference(object client.Object, owner *opsterv1.OpenSearchCluster) bool {
	for _, ownerRef := range object.GetOwnerReferences() {
		if ownerRef.Name == owner.ObjectMeta.Name {
			return true
		}
	}
	return false
}

func ArrayElementContains(array []string, content string) bool {
	for _, element := range array {
		if strings.Contains(element, content) {
			return true
		}
	}
	return false
}

func ComposeOpensearchCrd(clusterName string, namespace string) opsterv1.OpenSearchCluster {

	OpensearchCluster := &opsterv1.OpenSearchCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OpenSearchCluster",
			APIVersion: "opensearch.opster.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
				Monitoring:  opsterv1.MonitoringConfig{Enable: true, ScrapeInterval: "35s", TLSConfig: &opsterv1.MonitoringConfigTLS{InsecureSkipVerify: true, ServerName: "foo.bar"}},
				HttpPort:    9200,
				Vendor:      "opensearch",
				Version:     "2.0.0",
				ServiceName: "es-svc",
				PluginsList: []string{"http://foo-plugin-1.0.0"},
				AdditionalConfig: map[string]string{
					"foo": "bar",
				},
				AdditionalVolumes: []opsterv1.AdditionalVolume{
					{
						Name: "test-secret",
						Path: "/opt/test-secret",
						Secret: &corev1.SecretVolumeSource{
							SecretName: "test-secret",
						},
						RestartPods: false,
					},
					{
						Name:        "test-emptydir",
						Path:        "/tmp/",
						EmptyDir:    &corev1.EmptyDirVolumeSource{},
						RestartPods: false,
					},
					{
						Name: "test-cm",
						Path: "/opt/test-cm",
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-cm",
							},
						},
						RestartPods: true,
					},
				},
			},
			ConfMgmt: opsterv1.ConfMgmt{
				AutoScaler:  false,
				VerUpdate:   false,
				SmartScaler: false,
			},
			Bootstrap: opsterv1.BootstrapConfig{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("125m"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					}},
				Tolerations: []corev1.Toleration{{
					Effect:   "NoSchedule",
					Key:      "foo",
					Operator: "Equal",
					Value:    "bar",
				}},
			},
			Dashboards: opsterv1.DashboardsConfig{
				Enable:   true,
				Replicas: 3,
				Version:  "2.0.0",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					}},
				Tolerations: []corev1.Toleration{{
					Effect:   "NoSchedule",
					Key:      "foo",
					Operator: "Equal",
					Value:    "bar",
				}},
				NodeSelector: map[string]string{
					"foo": "bar",
				},
				Affinity: &corev1.Affinity{},
			},
			NodePools: []opsterv1.NodePool{{
				Component: "master",
				Replicas:  3,
				DiskSize:  "32Gi",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					}},
				Labels: map[string]string{
					"role": "master",
				},
				TopologySpreadConstraints: []corev1.TopologySpreadConstraint{{
					MaxSkew:           1,
					TopologyKey:       "zone",
					WhenUnsatisfiable: "DoNotSchedule",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"role": "master",
						},
					},
				}},
				Roles: []string{
					"master",
					"data",
				},
				Persistence: &opsterv1.PersistenceConfig{PersistenceSource: opsterv1.PersistenceSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			}, {
				Component: "nodes",
				Replicas:  3,
				DiskSize:  "32Gi",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					}},
				Roles: []string{
					"data",
				},
				Persistence: &opsterv1.PersistenceConfig{PersistenceSource: opsterv1.PersistenceSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			}, {
				Component: "client",
				Replicas:  3,
				DiskSize:  "32Gi",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					}},
				Roles: []string{
					"data",
					"ingest",
				},
				AdditionalConfig: map[string]string{
					"baz": "bat",
				},
				Labels: map[string]string{
					"quux": "quut",
				},
				Env: []corev1.EnvVar{
					{Name: "qux", Value: "qut"},
					{Name: "quuxe", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.labels['quux']"}}},
				},
				Persistence: &opsterv1.PersistenceConfig{PersistenceSource: opsterv1.PersistenceSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			}},
		},
		Status: opsterv1.ClusterStatus{
			ComponentsStatus: []opsterv1.ComponentStatus{{
				Component:   "",
				Status:      "",
				Description: "",
			},
			},
		},
	}
	return *OpensearchCluster

}
