package controllers

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

func IsNsCreated(k8sClient client.Client, namespace corev1.Namespace) bool {
	ns := corev1.Namespace{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: namespace.Name}, &ns); err == nil {
		return true
	} else {
		return false
	}
}

func IsClusterCreated(k8sClient client.Client, cluster opsterv1.OpenSearchCluster) bool {
	ns := corev1.Namespace{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, &ns); err == nil {
		return true
	} else {
		return false
	}
}

func ComposeOpensearchCrd(ClusterName string, ClusterNameSpaces string) opsterv1.OpenSearchCluster {

	OpensearchCluster := &opsterv1.OpenSearchCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OpenSearchCluster",
			APIVersion: "opensearch.opster.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterNameSpaces,
		},
		Spec: opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
				ClusterName: ClusterName,
				HttpPort:    9200,
				Vendor:      "opensearch",
				Version:     "latest",
				ServiceName: "es-svc",
			},
			ConfMgmt: opsterv1.ConfMgmt{
				AutoScaler: false,
				Monitoring: false,
				VerUpdate:  false,
			},
			Dashboards: opsterv1.DashboardsConfig{Enable: true},
			NodePools: []opsterv1.NodePool{{
				Component:    "master",
				Replicas:     3,
				DiskSize:     32,
				NodeSelector: "",
				Cpu:          4,
				Memory:       16,
				Roles: []string{
					"master",
					"data",
				}}, {
				Component:    "nodes",
				Replicas:     3,
				DiskSize:     32,
				NodeSelector: "",
				Cpu:          4,
				Memory:       16,
				Roles: []string{
					"data",
				}}, {
				Component:    "client",
				Replicas:     3,
				DiskSize:     32,
				NodeSelector: "",
				Cpu:          4,
				Memory:       16,
				Roles: []string{
					"data",
					"ingest",
				},
			}},
		},
		Status: opsterv1.ClusterStatus{ComponentsStatus: []opsterv1.ComponentStatus{}},
	}
	return *OpensearchCluster
}

func ComposeNs(name string) corev1.Namespace {

	return corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}
