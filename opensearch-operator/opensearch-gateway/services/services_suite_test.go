package services

/*

   import (
   	"fmt"
   	. "github.com/onsi/ginkgo"
   	. "github.com/onsi/gomega"
   	"k8s.io/utils/exec"
   	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
   	"path/filepath"
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

   var path = filepath.Join(helpers.GetOperatorRootPath(), "test_resources/docker-compose.yml")

   var _ = BeforeSuite(func() {
   	cmd := exec.New().Command("docker-compose", "-f", path, "up", "-d")

   	output, err := cmd.Output()
   	fmt.Println(string(output))
   	fmt.Println(err)
   	//Expect(err).NotTo(HaveOccurred())
   }, 60)

   var _ = AfterSuite(func() {
   	_, err := exec.New().Command("docker-compose", "-f", path, "down").Output()
   	if err != nil {
   		fmt.Println("failed to stop docker compose")
   	}
   })
*/
