package helpers

import (
	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("ClusterURL", func() {
	It("should use operatorClusterURL when provided", func() {
		customHost := "opensearch.example.com"
		cluster := &opsterv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec: opsterv1.ClusterSpec{
				General: opsterv1.GeneralConfig{
					OperatorClusterURL: &customHost,
					HttpPort:           9443,
					ServiceName:        "test",
				},
			},
		}

		result := ClusterURL(cluster)
		Expect(result).To(Equal("https://opensearch.example.com:9443"))
	})

	It("should use default internal DNS when operatorClusterURL is nil", func() {
		cluster := &opsterv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec: opsterv1.ClusterSpec{
				General: opsterv1.GeneralConfig{
					HttpPort:    9200,
					ServiceName: "test",
				},
			},
		}

		result := ClusterURL(cluster)
		Expect(result).To(Equal("https://test.default.svc.cluster.local:9200"))
	})

	It("should use default port 9200 when HttpPort is 0", func() {
		customHost := "opensearch.example.com"
		cluster := &opsterv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec: opsterv1.ClusterSpec{
				General: opsterv1.GeneralConfig{
					OperatorClusterURL: &customHost,
					ServiceName:        "test",
				},
			},
		}

		result := ClusterURL(cluster)
		Expect(result).To(Equal("https://opensearch.example.com:9200"))
	})
})

var _ = Describe("Helper Functions", func() {

	Describe("ResolveUidGid", func() {
		Context("when no security context is specified", func() {
			It("should return default values", func() {
				cluster := &opsterv1.OpenSearchCluster{
					Spec: opsterv1.ClusterSpec{
						General: opsterv1.GeneralConfig{},
					},
				}

				uid, gid := ResolveUidGid(cluster)
				Expect(uid).To(Equal(DefaultUID))
				Expect(gid).To(Equal(DefaultGID))
			})
		})

		Context("when only container security context is specified", func() {
			It("should use container security context values", func() {
				cluster := &opsterv1.OpenSearchCluster{
					Spec: opsterv1.ClusterSpec{
						General: opsterv1.GeneralConfig{
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:  ptr.To(int64(2000)),
								RunAsGroup: ptr.To(int64(2000)),
							},
						},
					},
				}

				uid, gid := ResolveUidGid(cluster)
				Expect(uid).To(Equal(int64(2000)))
				Expect(gid).To(Equal(int64(2000)))
			})
		})

		Context("when only pod security context is specified", func() {
			It("should use pod security context values", func() {
				cluster := &opsterv1.OpenSearchCluster{
					Spec: opsterv1.ClusterSpec{
						General: opsterv1.GeneralConfig{
							PodSecurityContext: &corev1.PodSecurityContext{
								RunAsUser:  ptr.To(int64(1500)),
								RunAsGroup: ptr.To(int64(1500)),
							},
						},
					},
				}

				uid, gid := ResolveUidGid(cluster)
				Expect(uid).To(Equal(int64(1500)))
				Expect(gid).To(Equal(int64(1500)))
			})
		})

		Context("when both security contexts are specified", func() {
			It("should prioritize container security context over pod security context", func() {
				cluster := &opsterv1.OpenSearchCluster{
					Spec: opsterv1.ClusterSpec{
						General: opsterv1.GeneralConfig{
							PodSecurityContext: &corev1.PodSecurityContext{
								RunAsUser:  ptr.To(int64(1500)),
								RunAsGroup: ptr.To(int64(1500)),
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:  ptr.To(int64(3000)),
								RunAsGroup: ptr.To(int64(3000)),
							},
						},
					},
				}

				uid, gid := ResolveUidGid(cluster)
				Expect(uid).To(Equal(int64(3000)))
				Expect(gid).To(Equal(int64(3000)))
			})
		})

		Context("when security contexts have partial values", func() {
			It("should use container UID and pod GID when container GID is missing", func() {
				cluster := &opsterv1.OpenSearchCluster{
					Spec: opsterv1.ClusterSpec{
						General: opsterv1.GeneralConfig{
							PodSecurityContext: &corev1.PodSecurityContext{
								RunAsUser:  ptr.To(int64(1500)),
								RunAsGroup: ptr.To(int64(1800)),
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: ptr.To(int64(2500)),
								// RunAsGroup not specified
							},
						},
					},
				}

				uid, gid := ResolveUidGid(cluster)
				Expect(uid).To(Equal(int64(2500))) // From container context
				Expect(gid).To(Equal(int64(1800))) // From pod context (fallback)
			})

			It("should use defaults when only empty security contexts are provided", func() {
				cluster := &opsterv1.OpenSearchCluster{
					Spec: opsterv1.ClusterSpec{
						General: opsterv1.GeneralConfig{
							PodSecurityContext: &corev1.PodSecurityContext{},
							SecurityContext:    &corev1.SecurityContext{},
						},
					},
				}

				uid, gid := ResolveUidGid(cluster)
				Expect(uid).To(Equal(DefaultUID))
				Expect(gid).To(Equal(DefaultGID))
			})
		})
	})

	Describe("GetChownCommand", func() {
		Context("with valid UID and GID", func() {
			It("should generate correct chown command with default values", func() {
				command := GetChownCommand(1000, 1000, "/usr/share/opensearch/data")
				Expect(command).To(Equal("chown -R 1000:1000 /usr/share/opensearch/data"))
			})
		})
	})

	Describe("MergeConfigs mutation behavior", func() {
		It("should merge the maps such that right is higher priority than left, and not mutate either argument when merging", func() {
			generalConfig := map[string]string{"http.compression": "true"}
			poolConfig := map[string]string{"node.data": "false"}

			// Save a copy of the original
			original := map[string]string{"http.compression": "true"}

			// Merge and check result
			merged := MergeConfigs(generalConfig, poolConfig)
			expected := map[string]string{"http.compression": "true", "node.data": "false"}
			Expect(merged).To(Equal(expected))

			// Check that longLived was not mutated
			Expect(generalConfig).To(Equal(original))

			// Merge again with a new config
			poolConfig2 := map[string]string{"node.master": "false", "http.compression": "false"}
			expected2 := map[string]string{"http.compression": "false", "node.master": "false"}
			merged2 := MergeConfigs(generalConfig, poolConfig2)
			Expect(merged2).To(Equal(expected2))

			// Still not mutated
			Expect(generalConfig).To(Equal(original))
		})
	})

})
