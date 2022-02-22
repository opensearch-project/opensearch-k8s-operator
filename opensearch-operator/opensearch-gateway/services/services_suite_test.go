package services

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
	opsterv1 "opensearch.opster.io/api/v1"
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
	TestClusterUrl      = "https://localhost:9111"
	TestClusterUserName = "admin"
	TestClusterPassword = "admin"
)

var _ = BeforeSuite(func() {
	helpers.BeforeSuiteLogic()

	cmd := exec.New().Command("docker-compose", "-f", "../../test_resources/docker-compose.yml", "up", "-d")
	_, err := cmd.Output()
	if err != nil {
		fmt.Println("failed to start tests. please make sure docker compose is installed and configured in path")
	}
}, 60)

var _ = AfterSuite(func() {
	helpers.AfterSuiteLogic()
	exec.New().Command("docker-compose", "-f", "../../test_resources/docker-compose.yml", "down")

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
