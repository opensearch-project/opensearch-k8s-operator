package builders

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	opsterv1 "opensearch.opster.io/api/v1"
)

var _ = Describe("Dashboards", func() {

	cluster := opsterv1.OpenSearchCluster{
		Spec: opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{},
			Dashboards: opsterv1.DashboardsConfig{
				Enable: true,
			},
		},
	}

	When("enabling dashboards", func() {
		It("it should create a service", func() {
			Expect(cluster.Spec.Dashboards.Service).NotTo(Equal(nil))
		})

		It("should reflect service type being changed", func() {
			t := corev1.ServiceTypeLoadBalancer
			cluster.Spec.Dashboards.Service.Type = t
			Expect(cluster.Spec.Dashboards.Service.Type).To(Equal(t))
		})
	})

})
