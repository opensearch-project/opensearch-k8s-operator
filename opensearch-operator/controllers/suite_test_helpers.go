package controllers

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
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
				HttpPort:    9200,
				Vendor:      "opensearch",
				Version:     "latest",
				ServiceName: "es-svc",
			},
			ConfMgmt: opsterv1.ConfMgmt{
				AutoScaler:  false,
				Monitoring:  false,
				VerUpdate:   false,
				SmartScaler: false,
			},
			Dashboards: opsterv1.DashboardsConfig{Enable: true},
			NodePools: []opsterv1.NodePool{{
				Component: "master",
				Replicas:  3,
				DiskSize:  32,
				Resources: corev1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("500m"),
						v1.ResourceMemory: resource.MustParse("2Gi"),
					}},
				Roles: []string{
					"master",
					"data",
				}}, {
				Component: "nodes",
				Replicas:  3,
				DiskSize:  32,
				Resources: corev1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("500m"),
						v1.ResourceMemory: resource.MustParse("2Gi"),
					}},
				Roles: []string{
					"data",
				}}, {
				Component: "client",
				Replicas:  3,
				DiskSize:  32,
				Resources: corev1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("500m"),
						v1.ResourceMemory: resource.MustParse("2Gi"),
					}},
				Roles: []string{
					"data",
					"ingest",
				},
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
