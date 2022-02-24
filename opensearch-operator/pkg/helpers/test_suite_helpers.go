package helpers

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

var (
	K8sClient client.Client // You'll be using this client in your tests.
)

func IsNsDeleted(k8sClient client.Client, namespace corev1.Namespace) bool {
	ns := corev1.Namespace{}
	if err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: namespace.Name, Namespace: namespace.Namespace}, &ns); err == nil {
		return ns.Status.Phase == "Terminating"
	}
	return true
}

func IsNsCreated(k8sClient client.Client, namespace corev1.Namespace) bool {
	ns := corev1.Namespace{}
	if err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: namespace.Name}, &ns); err == nil {
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

func GetOperatorRootPath() string {
	path, _ := os.Getwd()
	index := strings.Index(path, "opensearch-k8-operator/opensearch-operator")
	length := len("opensearch-k8s-operator/opensearch-operator")
	if index < 0 {
		return ""
	}

	inputFmt := path[0 : index+length]
	return inputFmt

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
				}}},
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

func ComposeNs(name string) corev1.Namespace {

	return corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}
