package builders

import (
	"fmt"

	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Builders", func() {
	When("building the dashboards deployment with annotations supplied", func() {
		It("should populate the dashboard pod and deployment spec with annotations provided", func() {
			clusterName := "dashboards-add-annotations"
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opensearchv1.DashboardsConfig{
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
			Expect(result.ObjectMeta.Annotations).To(Equal(map[string]string{
				"testAnnotationKey":  "testValue",
				"testAnnotationKey2": "testValue2",
			}))
		})
	})

	When("building the dashboards deployment with labels supplied", func() {
		It("should populate the dashboard pod spec with labels provided", func() {
			clusterName := "dashboards-add-labels"
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opensearchv1.DashboardsConfig{
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
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opensearchv1.DashboardsConfig{
						Enable: true,
						Annotations: map[string]string{
							"testAnnotationKey":  "testValue",
							"testAnnotationKey2": "testValue2",
						},
						Service: opensearchv1.DashboardsServiceSpec{
							Type:                     "LoadBalancer",
							LoadBalancerSourceRanges: sourceRanges,
						},
					},
				},
			}
			result := NewDashboardsSvcForCr(&spec, spec.Spec.Dashboards.Service.Labels)
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

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "some-name", Namespace: "some-namespace", UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{ServiceName: "some-name"},
					Dashboards: opensearchv1.DashboardsConfig{
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
				RunAsNonRoot: ptr.To(true),
			}
			securityContext := &corev1.SecurityContext{
				Privileged:               ptr.To(false),
				AllowPrivilegeEscalation: ptr.To(false),
			}
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "some-name", Namespace: "some-namespace", UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{ServiceName: "some-name"},
					Dashboards: opensearchv1.DashboardsConfig{
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
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "some-name", Namespace: "some-namespace", UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						ServiceName:    "some-name",
						ServiceAccount: serviceAccountName,
					},
					Dashboards: opensearchv1.DashboardsConfig{
						Enable: true,
					},
				},
			}
			result := NewDashboardsDeploymentForCR(&spec, nil, nil, nil)
			Expect(result.Spec.Template.Spec.ServiceAccountName).To(Equal(serviceAccountName))
		})
	})

	When("building the dashboards service with custom service labels supplied", func() {
		It("should populate the service metadata.labels with the supplied service labels only", func() {
			clusterName := "dashboards-service-labels"
			serviceLabels := map[string]string{
				"monitoring": "enabled",
				"team":       "search",
			}
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opensearchv1.DashboardsConfig{
						Enable: true,
						Service: opensearchv1.DashboardsServiceSpec{
							Type:   "ClusterIP",
							Labels: serviceLabels,
						},
					},
				},
			}

			result := NewDashboardsSvcForCr(&spec, serviceLabels)

			expectedMetadataLabels := map[string]string{
				"opensearch.cluster.dashboards": clusterName,
				"monitoring":                    "enabled",
				"team":                          "search",
			}
			expectedSelectorLabels := map[string]string{
				"opensearch.cluster.dashboards": clusterName,
			}

			Expect(result.ObjectMeta.Labels).To(Equal(expectedMetadataLabels))
			Expect(result.Spec.Selector).To(Equal(expectedSelectorLabels))
		})
	})

	When("building the dashboards service without service labels", func() {
		It("should default to only the dashboard selector label", func() {
			clusterName := "dashboards-no-service-labels"
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{ServiceName: clusterName},
					Dashboards: opensearchv1.DashboardsConfig{
						Enable: true,
						Service: opensearchv1.DashboardsServiceSpec{
							Type: "ClusterIP",
							// Labels is nil
						},
					},
				},
			}

			result := NewDashboardsSvcForCr(&spec, nil)

			Expect(result.ObjectMeta.Labels).To(HaveKeyWithValue("opensearch.cluster.dashboards", clusterName))
			Expect(result.Spec.Selector).To(HaveKeyWithValue("opensearch.cluster.dashboards", clusterName))
		})
	})

	When("configuring a host alias for the dashboards", func() {
		It("should configure the host alias for the dashboard pods", func() {
			hostNames := []string{"dummy.com"}
			hostAlias := corev1.HostAlias{
				IP:        "3.5.7.9",
				Hostnames: hostNames,
			}
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "some-name", Namespace: "some-namespace", UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						ServiceName: "some-name",
					},
					Dashboards: opensearchv1.DashboardsConfig{
						Enable:      true,
						HostAliases: []corev1.HostAlias{hostAlias},
					},
				},
			}
			result := NewDashboardsDeploymentForCR(&spec, nil, nil, nil)
			Expect(result.Spec.Template.Spec.HostAliases).To(Equal([]corev1.HostAlias{hostAlias}))
		})
	})

	When("configuring hostNetwork for the dashboards", func() {
		It("should set hostNetwork on the dashboard pods when enabled", func() {
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "some-name", Namespace: "some-namespace", UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						ServiceName: "some-name",
						HostNetwork: true,
					},
					Dashboards: opensearchv1.DashboardsConfig{
						Enable: true,
					},
				},
			}
			result := NewDashboardsDeploymentForCR(&spec, nil, nil, nil)
			Expect(result.Spec.Template.Spec.HostNetwork).To(BeTrue())
		})

		It("should not set hostNetwork on the dashboard pods when disabled", func() {
			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "some-name", Namespace: "some-namespace", UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						ServiceName: "some-name",
						HostNetwork: false,
					},
					Dashboards: opensearchv1.DashboardsConfig{
						Enable: true,
					},
				},
			}
			result := NewDashboardsDeploymentForCR(&spec, nil, nil, nil)
			Expect(result.Spec.Template.Spec.HostNetwork).To(BeFalse())
		})
	})

	When("building the dashboards probes", func() {
		newSpec := func(dashboards opensearchv1.DashboardsConfig) *opensearchv1.OpenSearchCluster {
			dashboards.Enable = true
			return &opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "some-name", Namespace: "some-namespace", UID: "dummyuid"},
				Spec: opensearchv1.ClusterSpec{
					General:    opensearchv1.GeneralConfig{ServiceName: "some-name"},
					Dashboards: dashboards,
				},
			}
		}
		timings := func(p *corev1.Probe) []int32 {
			return []int32{p.InitialDelaySeconds, p.PeriodSeconds, p.TimeoutSeconds, p.SuccessThreshold, p.FailureThreshold}
		}

		It("should give the startup probe a longer budget than liveness and readiness by default", func() {
			container := NewDashboardsDeploymentForCR(newSpec(opensearchv1.DashboardsConfig{}), nil, nil, nil).Spec.Template.Spec.Containers[0]
			Expect(timings(container.StartupProbe)).To(Equal([]int32{10, 20, 5, 1, 60}))
			Expect(timings(container.LivenessProbe)).To(Equal([]int32{10, 20, 5, 1, 10}))
			Expect(timings(container.ReadinessProbe)).To(Equal([]int32{10, 20, 5, 1, 10}))
			Expect(container.StartupProbe.HTTPGet.Path).To(Equal("/api/reporting/stats"))
			Expect(container.StartupProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTP))
		})

		It("should apply each probe's overrides only to that probe and keep defaults for unset fields", func() {
			spec := newSpec(opensearchv1.DashboardsConfig{
				Probes: &opensearchv1.DashboardsProbesConfig{
					Startup:   &opensearchv1.ProbeConfig{FailureThreshold: 90},
					Liveness:  &opensearchv1.ProbeConfig{InitialDelaySeconds: 30, PeriodSeconds: 15, TimeoutSeconds: 8, SuccessThreshold: 1, FailureThreshold: 4},
					Readiness: &opensearchv1.ProbeConfig{PeriodSeconds: 10, SuccessThreshold: 2},
				},
			})
			container := NewDashboardsDeploymentForCR(spec, nil, nil, nil).Spec.Template.Spec.Containers[0]
			Expect(timings(container.StartupProbe)).To(Equal([]int32{10, 20, 5, 1, 90}))
			Expect(timings(container.LivenessProbe)).To(Equal([]int32{30, 15, 8, 1, 4}))
			Expect(timings(container.ReadinessProbe)).To(Equal([]int32{10, 10, 5, 2, 10}))
		})

		It("should keep the defaults for probes left out of the probes config", func() {
			spec := newSpec(opensearchv1.DashboardsConfig{
				Probes: &opensearchv1.DashboardsProbesConfig{
					Liveness: &opensearchv1.ProbeConfig{FailureThreshold: 3},
				},
			})
			container := NewDashboardsDeploymentForCR(spec, nil, nil, nil).Spec.Template.Spec.Containers[0]
			Expect(timings(container.StartupProbe)).To(Equal([]int32{10, 20, 5, 1, 60}))
			Expect(timings(container.LivenessProbe)).To(Equal([]int32{10, 20, 5, 1, 3}))
			Expect(timings(container.ReadinessProbe)).To(Equal([]int32{10, 20, 5, 1, 10}))
		})

		It("should keep the base path and HTTPS scheme on every probe", func() {
			spec := newSpec(opensearchv1.DashboardsConfig{
				BasePath: "/dashboards",
				Tls:      &opensearchv1.DashboardsTlsConfig{Enable: true},
				Probes: &opensearchv1.DashboardsProbesConfig{
					Startup: &opensearchv1.ProbeConfig{FailureThreshold: 90},
				},
			})
			container := NewDashboardsDeploymentForCR(spec, nil, nil, nil).Spec.Template.Spec.Containers[0]
			for _, probe := range []*corev1.Probe{container.StartupProbe, container.LivenessProbe, container.ReadinessProbe} {
				Expect(probe.HTTPGet.Path).To(Equal("/dashboards/api/reporting/stats"))
				Expect(probe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))
				Expect(probe.HTTPGet.Port.IntVal).To(Equal(int32(5601)))
			}
		})
	})
})
