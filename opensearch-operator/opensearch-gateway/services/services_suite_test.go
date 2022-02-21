package services

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/builders"
	"opensearch.opster.io/pkg/helpers"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	"testing"
)

func TestServices(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"Services Suite",
		[]Reporter{printer.NewlineReporter{}})
}

const (
	ClusterName = "cluster-test-nodes"
	NameSpace   = "default"
	CmName      = "opensearch-yml"
)

var (
	OpensearchCluster   = ComposeOpensearchCrdForServices(ClusterName, NameSpace)
	Ns                  = builders.NewNsForCR(OpensearchCluster)
	HeadLessServiceName = builders.HeadlessName(OpensearchCluster)
	ServiceName         = builders.ServiceName(OpensearchCluster)
	StsName             = builders.StsName(OpensearchCluster, OpensearchCluster.Spec.NodePools[0])
)

var _ = BeforeSuite(func() {
	helpers.BeforeSuiteLogic()

	if !helpers.IsNsCreated(helpers.K8sClient, context.TODO(), *Ns) {
		err := helpers.K8sClient.Create(context.TODO(), Ns)
		if err != nil {
			return
		}
		if !helpers.IsNsCreated(helpers.K8sClient, context.TODO(), *Ns) {
			return
		}
	}
	if !helpers.IsCmCreated(helpers.K8sClient, CmName, ClusterName) {
		clusterCm := builders.NewCmForCR(OpensearchCluster)
		err := helpers.K8sClient.Create(context.TODO(), clusterCm)
		if err != nil {
			return
		}
		if !helpers.IsCmCreated(helpers.K8sClient, CmName, ClusterName) {
			return
		}
	}
	if !helpers.IsServiceCreated(helpers.K8sClient, HeadLessServiceName, ClusterName) {
		headless_service := builders.NewHeadlessServiceForCR(OpensearchCluster)
		err := helpers.K8sClient.Create(context.TODO(), headless_service)
		if err != nil {
			return
		}
		if !helpers.IsServiceCreated(helpers.K8sClient, HeadLessServiceName, ClusterName) {
			return
		}
	}
	if !helpers.IsServiceCreated(helpers.K8sClient, ServiceName, ClusterName) {
		service := builders.NewServiceForCR(OpensearchCluster)
		err := helpers.K8sClient.Create(context.TODO(), service)
		if err != nil {
			return
		}
		if !helpers.IsServiceCreated(helpers.K8sClient, ServiceName, ClusterName) {
			return
		}
	}
	if !helpers.IsStsCreated(helpers.K8sClient, StsName, ClusterName) {
		sts := builders.NewSTSForCR(OpensearchCluster, OpensearchCluster.Spec.NodePools[0])
		err := helpers.K8sClient.Create(context.TODO(), sts)
		if err != nil {
			return
		}
		if !helpers.IsStsCreated(helpers.K8sClient, StsName, ClusterName) {
			return
		}
	}

}, 60)

var _ = AfterSuite(func() {
	helpers.AfterSuiteLogic()
})

func ComposeOpensearchCrdForServices(ClusterName string, ClusterNameSpaces string) *opsterv1.OpenSearchCluster {
	return &opsterv1.OpenSearchCluster{
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
				}},
			},
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

}
