package builders

import (
	"fmt"

	"k8s.io/utils/pointer"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				},
			}
			result := NewDashboardsDeploymentForCR(&spec, nil, nil, nil)
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
				},
			}
			result := NewDashboardsDeploymentForCR(&spec, nil, nil, nil)
			Expect(result.Spec.Template.Labels).To(Equal(map[string]string{
				"opensearch.cluster.dashboards": clusterName,
				"testLabelKey":                  "testValue",
				"testLabelKey2":                 "testValue2",
			}))
		})
	})

	When("building the dashboards deployment with a custom service type", func() {
		It("should populate the service with the correct type and source ranges", func() {
			clusterName := "dashboards-add-service-type-load-balancer"
			sourceRanges := []string{"10.0.0.0/24"}
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
						Service: opsterv1.DashboardsServiceSpec{
							Type:                     "LoadBalancer",
							LoadBalancerSourceRanges: sourceRanges,
						},
					},
				},
			}
			result := NewDashboardsSvcForCr(&spec)
			Expect(result.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
			Expect(result.Spec.LoadBalancerSourceRanges).To(Equal(sourceRanges))
			Expect(result.Annotations).To(Equal(map[string]string{
				"testAnnotationKey":  "testValue",
				"testAnnotationKey2": "testValue2",
			}))
		})
	})

	When("building the dashboards deployment with plugins that should be installed", func() {
		It("should properly setup the main command when installing plugins", func() {
			pluginA := "some-plugin"
			pluginB := "another-plugin"

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "some-name", Namespace: "some-namespace", UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{ServiceName: "some-name"},
					Dashboards: opsterv1.DashboardsConfig{
						Enable:      true,
						PluginsList: []string{pluginA, pluginB},
					},
				},
			}

			result := NewDashboardsDeploymentForCR(&spec, nil, nil, nil)
			installCmd := fmt.Sprintf(
				"./bin/opensearch-dashboards-plugin install '%s' && ./bin/opensearch-dashboards-plugin install '%s' && ./opensearch-dashboards-docker-entrypoint.sh",
				pluginA,
				pluginB,
			)
			expected := []string{
				"/bin/bash",
				"-c",
				installCmd,
			}
			actual := result.Spec.Template.Spec.Containers[0].Command

			Expect(expected).To(Equal(actual))
		})
	})

	When("building the dashboards deployment with security contexts set", func() {
		It("should populate the dashboard pod spec with security contexts provided", func() {
			user := int64(1000)
			podSecurityContext := &corev1.PodSecurityContext{
				RunAsUser:    &user,
				RunAsGroup:   &user,
				RunAsNonRoot: pointer.Bool(true),
			}
			securityContext := &corev1.SecurityContext{
				Privileged:               pointer.Bool(false),
				AllowPrivilegeEscalation: pointer.Bool(false),
			}
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "some-name", Namespace: "some-namespace", UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{ServiceName: "some-name"},
					Dashboards: opsterv1.DashboardsConfig{
						Enable:             true,
						PodSecurityContext: podSecurityContext,
						SecurityContext:    securityContext,
					},
				},
			}
			result := NewDashboardsDeploymentForCR(&spec, nil, nil, nil)
			Expect(result.Spec.Template.Spec.SecurityContext).To(Equal(podSecurityContext))
			Expect(result.Spec.Template.Spec.Containers[0].SecurityContext).To(Equal(securityContext))
		})
	})

	When("configuring a serviceaccount for the cluster", func() {
		It("should configure the serviceaccount for the dashboard pods", func() {
			const serviceAccountName = "my-serviceaccount"
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "some-name", Namespace: "some-namespace", UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{
						ServiceName:    "some-name",
						ServiceAccount: serviceAccountName,
					},
					Dashboards: opsterv1.DashboardsConfig{
						Enable: true,
					},
				},
			}
			result := NewDashboardsDeploymentForCR(&spec, nil, nil, nil)
			Expect(result.Spec.Template.Spec.ServiceAccountName).To(Equal(serviceAccountName))
		})
	})
})
