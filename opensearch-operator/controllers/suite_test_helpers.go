package controllers

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	//. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	timeout  = time.Second * 30
	interval = time.Millisecond * 250
)

func IsCreated(ctx context.Context, k8sClient client.Client, obj client.Object) {
	EventuallyWithOffset(1, func() bool {
		if err := k8sClient.Get(ctx, client.ObjectKey{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		}, obj); err != nil {
			return false
		}
		return true
	}, timeout, interval).Should(BeTrue())
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
				ClusterName: "default",
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
				Replicas:     5,
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
		Status: opsterv1.ClusterStatus{ComponentsStatus: nil},
	}
	return *OpensearchCluster
}
