package util

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	opsterv1 "opensearch.opster.io/api/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestUtil(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Util Suite")
}

var _ = BeforeSuite(func() {

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	err := scheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = opsterv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
})
