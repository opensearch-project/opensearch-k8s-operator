package services

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/exec"
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
