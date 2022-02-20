package helpers

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestOsClient(t *testing.T) {
	RegisterFailHandler(Fail)
	/*RunSpecsWithDefaultAndCustomReporters(t,
	"Controller Suite",
	[]Reporter{printer.NewlineReporter{}})*/
	RunSpecs(t, "Tests")

}

var _ = BeforeSuite(func() {
	BeforeSuiteLogic()

}, 60)

var _ = AfterSuite(func() {
	AfterSuiteLogic()
})
