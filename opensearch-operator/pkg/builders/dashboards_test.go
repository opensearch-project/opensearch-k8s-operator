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

	When("When enabling dashboards", func() {
		It("it should create a service", func() {
			Expect(cluster.Spec.Dashboards.Service).NotTo(Equal(nil))
		})

		It("updating the service type through the cluster spec should reflect the service", func() {
			t := corev1.ServiceTypeLoadBalancer
			cluster.Spec.Dashboards.Service.Type = t
			Expect(cluster.Spec.Dashboards.Service.Type).To(Equal(t))
		})
	})

})
