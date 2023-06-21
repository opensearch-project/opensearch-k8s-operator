package operatortests

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("dummy", Ordered, func() {
	name := "dummy"
	namespace := "default"

	BeforeAll(func() {
		CreateKubernetesObjects(name)
	})

	It("should have the secret", func() {
		secret := corev1.Secret{}
		Get(&secret, client.ObjectKey{Name: "dummy", Namespace: namespace}, time.Second*5)
		data, ok := secret.Data["foo"]
		Expect(ok).To(BeTrue())
		Expect(string(data)).To(Equal("bar"))
	})

	AfterAll(func() {
		Cleanup(name)
	})
})
