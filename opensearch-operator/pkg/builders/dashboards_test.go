package builders

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"
)

var _ = Describe("Builders", func() {
	When("building the dashboards deployment with annotations supplied", func() {
		It("should populate the dashboard pod spec with annotations provided", func() {
			clusterName := "dashboards-add-annotations"
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opsterv1.DashboardsConfig{
						Enable: true,
						Annotations: map[string]string{
							"testAnnotationKey":  "testValue",
							"testAnnotationKey2": "testValue2",
						},
					},
				}}
			var result = NewDashboardsDeploymentForCR(&spec, nil, nil, nil)
			Expect(result.Spec.Template.Annotations).To(Equal(map[string]string{
				"testAnnotationKey":  "testValue",
				"testAnnotationKey2": "testValue2",
			}))
		})
	})
	When("building the dashboards deployment with labels supplied", func() {
		It("should populate the dashboard pod spec with labels provided", func() {
			clusterName := "dashboards-add-labels"
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opsterv1.DashboardsConfig{
						Enable: true,
						Labels: map[string]string{
							"testLabelKey":  "testValue",
							"testLabelKey2": "testValue2",
						},
					},
				}}
			var result = NewDashboardsDeploymentForCR(&spec, nil, nil, nil)
			Expect(result.Spec.Template.Labels).To(Equal(map[string]string{
				"testLabelKey":  "testValue",
				"testLabelKey2": "testValue2",
			}))
		})
	})
})
