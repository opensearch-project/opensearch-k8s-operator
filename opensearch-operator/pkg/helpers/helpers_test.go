package helpers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	k8smocks "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("ClusterURL", func() {
	It("should use operatorClusterURL when provided", func() {
		customHost := "opensearch.example.com"
		cluster := &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{
					OperatorClusterURL: &customHost,
					HttpPort:           9443,
					ServiceName:        "test",
				},
			},
		}

		result := ClusterURL(cluster)
		Expect(result).To(Equal("http://opensearch.example.com:9443"))
	})

	It("should use default internal DNS when operatorClusterURL is nil", func() {
		cluster := &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{
					HttpPort:    9200,
					ServiceName: "test",
				},
			},
		}

		result := ClusterURL(cluster)
		Expect(result).To(Equal("http://test.default.svc.cluster.local:9200"))
	})

	It("should use default port 9200 when HttpPort is 0", func() {
		customHost := "opensearch.example.com"
		cluster := &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{
					OperatorClusterURL: &customHost,
					ServiceName:        "test",
				},
			},
		}

		result := ClusterURL(cluster)
		Expect(result).To(Equal("http://opensearch.example.com:9200"))
	})
})

var _ = Describe("Helper Functions", func() {

	Describe("ResolveUidGid", func() {
		Context("when no security context is specified", func() {
			It("should return default values", func() {
				cluster := &opensearchv1.OpenSearchCluster{
					Spec: opensearchv1.ClusterSpec{
						General: opensearchv1.GeneralConfig{},
					},
				}

				uid, gid := ResolveUidGid(cluster)
				Expect(uid).To(Equal(DefaultUID))
				Expect(gid).To(Equal(DefaultGID))
			})
		})

		Context("when only container security context is specified", func() {
			It("should use container security context values", func() {
				cluster := &opensearchv1.OpenSearchCluster{
					Spec: opensearchv1.ClusterSpec{
						General: opensearchv1.GeneralConfig{
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
				cluster := &opensearchv1.OpenSearchCluster{
					Spec: opensearchv1.ClusterSpec{
						General: opensearchv1.GeneralConfig{
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
				cluster := &opensearchv1.OpenSearchCluster{
					Spec: opensearchv1.ClusterSpec{
						General: opensearchv1.GeneralConfig{
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
				cluster := &opensearchv1.OpenSearchCluster{
					Spec: opensearchv1.ClusterSpec{
						General: opensearchv1.GeneralConfig{
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
				cluster := &opensearchv1.OpenSearchCluster{
					Spec: opensearchv1.ClusterSpec{
						General: opensearchv1.GeneralConfig{
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

var _ = Describe("JVM Heap Size Functions", func() {
	Describe("AppendJvmHeapSizeSettings", func() {
		Context("when JVM string already contains Xmx", func() {
			It("should return the original JVM string unchanged", func() {
				jvm := "-XX:+UseG1GC -Xmx2g -XX:MaxDirectMemorySize=1g"
				heapSizeSettings := "-Xms1g -Xmx2g"

				result := AppendJvmHeapSizeSettings(jvm, heapSizeSettings)

				Expect(result).To(Equal(jvm))
			})
		})

		Context("when JVM string already contains Xms", func() {
			It("should return the original JVM string unchanged", func() {
				jvm := "-XX:+UseG1GC -Xms1g -XX:MaxDirectMemorySize=1g"
				heapSizeSettings := "-Xms1g -Xmx2g"

				result := AppendJvmHeapSizeSettings(jvm, heapSizeSettings)

				Expect(result).To(Equal(jvm))
			})
		})

		Context("when JVM string is empty", func() {
			It("should return only the heap size settings", func() {
				jvm := ""
				heapSizeSettings := "-Xmx1g -Xms1g"

				result := AppendJvmHeapSizeSettings(jvm, heapSizeSettings)

				Expect(result).To(Equal(heapSizeSettings))
			})
		})

		Context("when JVM string does not contain Xmx or Xms", func() {
			It("should append the heap size settings", func() {
				jvm := "-XX:+UseG1GC -XX:MaxDirectMemorySize=1g"
				heapSizeSettings := "-Xmx1g -Xms1g"
				expected := "-XX:+UseG1GC -XX:MaxDirectMemorySize=1g -Xmx1g -Xms1g"

				result := AppendJvmHeapSizeSettings(jvm, heapSizeSettings)

				Expect(result).To(Equal(expected))
			})
		})
	})

	Describe("CalculateJvmHeapSizeSettings", func() {
		Context("when memory request is nil", func() {
			It("should return default 512M for both Xms and Xmx", func() {
				result := CalculateJvmHeapSizeSettings(nil)

				Expect(result).To(Equal("-Xms512M -Xmx512M"))
			})
		})

		Context("when memory request is zero", func() {
			It("should return default 512M for both Xms and Xmx", func() {
				memoryRequest := resource.MustParse("0")

				result := CalculateJvmHeapSizeSettings(&memoryRequest)

				Expect(result).To(Equal("-Xms512M -Xmx512M"))
			})
		})

		Context("when memory request is provided", func() {
			It("should calculate both Xms and Xmx from request", func() {
				memoryRequest := resource.MustParse("2Gi")

				result := CalculateJvmHeapSizeSettings(&memoryRequest)

				Expect(result).To(Equal("-Xms1024M -Xmx1024M"))
			})
		})
	})
})

var _ = Describe("TlsCASecretRef", func() {
	It("should return HTTP caSecret for OpenSearch 2.x", func() {
		cluster := &opensearchv1.OpenSearchCluster{
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{Version: "2.3.0"},
				Security: &opensearchv1.Security{
					Tls: &opensearchv1.TlsConfig{
						Http: &opensearchv1.TlsConfigHttp{
							TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
								CaSecret: corev1.LocalObjectReference{Name: "http-ca"},
							},
						},
					},
				},
			},
		}

		Expect(TlsCASecretRef(cluster).Name).To(Equal("http-ca"))
	})

	It("should return transport caSecret for OpenSearch 1.x", func() {
		cluster := &opensearchv1.OpenSearchCluster{
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{Version: "1.3.0"},
				Security: &opensearchv1.Security{
					Tls: &opensearchv1.TlsConfig{
						Transport: &opensearchv1.TlsConfigTransport{
							TlsCertificateConfig: opensearchv1.TlsCertificateConfig{
								CaSecret: corev1.LocalObjectReference{Name: "transport-ca"},
							},
						},
					},
				},
			},
		}

		Expect(TlsCASecretRef(cluster).Name).To(Equal("transport-ca"))
	})

	It("should return empty when TLS is not configured", func() {
		cluster := &opensearchv1.OpenSearchCluster{
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{Version: "2.3.0"},
			},
		}

		Expect(TlsCASecretRef(cluster).Name).To(BeEmpty())
	})
})

var _ = Describe("applyUserHashes", func() {
	It("should preserve custom users alongside admin and kibanaserver", func() {
		inputYaml := `
_meta:
  type: "internalusers"
  config_version: 2
admin:
  hash: "oldadminhash"
  reserved: true
  backend_roles:
    - "admin"
  description: "Admin user"
kibanaserver:
  hash: "oldkibanahash"
  reserved: true
  description: "Demo user for the OpenSearch Dashboards server"
readall:
  hash: "$2a$12$readallhash"
  reserved: false
  backend_roles:
    - "readall"
  description: "readall user"
snapshotrestore:
  hash: "$2a$12$snapshothash"
  reserved: false
  backend_roles:
    - "snapshotrestore"
  description: "snapshotrestore user"
`
		result, err := applyUserHashes(
			[]byte(inputYaml),
			[]byte("adminpass"),
			"",
			[]byte("dashboardspass"),
			"",
		)
		Expect(err).NotTo(HaveOccurred())

		var output map[string]interface{}
		err = yaml.Unmarshal(result, &output)
		Expect(err).NotTo(HaveOccurred())

		// admin and kibanaserver must still exist
		Expect(output).To(HaveKey("admin"))
		Expect(output).To(HaveKey("kibanaserver"))

		// Custom users must be preserved
		Expect(output).To(HaveKey("readall"))
		Expect(output).To(HaveKey("snapshotrestore"))

		// Verify custom user data is intact
		readall, ok := output["readall"].(map[interface{}]interface{})
		Expect(ok).To(BeTrue())
		Expect(readall["hash"]).To(Equal("$2a$12$readallhash"))
		Expect(readall["description"]).To(Equal("readall user"))

		snapshotrestore, ok := output["snapshotrestore"].(map[interface{}]interface{})
		Expect(ok).To(BeTrue())
		Expect(snapshotrestore["hash"]).To(Equal("$2a$12$snapshothash"))
		Expect(snapshotrestore["description"]).To(Equal("snapshotrestore user"))
	})

	It("should update admin and kibanaserver hashes while keeping custom users", func() {
		inputYaml := `
_meta:
  type: "internalusers"
  config_version: 2
admin:
  hash: "oldadminhash"
  reserved: true
  backend_roles:
    - "admin"
kibanaserver:
  hash: "oldkibanahash"
  reserved: true
  description: "Demo user for the OpenSearch Dashboards server"
customuser:
  hash: "$2a$12$customhash"
  reserved: false
  backend_roles:
    - "custom_role"
  description: "A custom user"
`
		adminHashOverride := "$2a$12$overriddenadminhash"
		dashboardsHashOverride := "$2a$12$overriddendashboardshash"

		result, err := applyUserHashes(
			[]byte(inputYaml),
			[]byte("adminpass"),
			adminHashOverride,
			[]byte("dashboardspass"),
			dashboardsHashOverride,
		)
		Expect(err).NotTo(HaveOccurred())

		var output map[string]interface{}
		err = yaml.Unmarshal(result, &output)
		Expect(err).NotTo(HaveOccurred())

		// Admin hash should be updated
		admin, ok := output["admin"].(map[interface{}]interface{})
		Expect(ok).To(BeTrue())
		Expect(admin["hash"]).To(Equal(adminHashOverride))

		// Kibanaserver hash should be updated
		kibana, ok := output["kibanaserver"].(map[interface{}]interface{})
		Expect(ok).To(BeTrue())
		Expect(kibana["hash"]).To(Equal(dashboardsHashOverride))

		// Custom user must still exist with original data
		custom, ok := output["customuser"].(map[interface{}]interface{})
		Expect(ok).To(BeTrue())
		Expect(custom["hash"]).To(Equal("$2a$12$customhash"))
		Expect(custom["description"]).To(Equal("A custom user"))
	})

	It("should preserve multiple custom users with various configurations", func() {
		inputYaml := `
_meta:
  type: "internalusers"
  config_version: 2
admin:
  hash: "adminhash"
  reserved: true
  backend_roles:
    - "admin"
kibanaserver:
  hash: "kibanahash"
  reserved: true
  description: "Demo user for the OpenSearch Dashboards server"
logstash:
  hash: "$2a$12$logstashhash"
  reserved: false
  backend_roles:
    - "logstash"
  description: "Logstash user"
kibanaro:
  hash: "$2a$12$kibanarohash"
  reserved: false
  backend_roles:
    - "kibanauser"
    - "readall"
  attributes:
    attribute1: "value1"
  description: "kibanaro user"
`
		result, err := applyUserHashes(
			[]byte(inputYaml),
			[]byte("adminpass"),
			"$2a$12$newhash",
			[]byte("dashboardspass"),
			"$2a$12$newkibanahash",
		)
		Expect(err).NotTo(HaveOccurred())

		var output map[string]interface{}
		err = yaml.Unmarshal(result, &output)
		Expect(err).NotTo(HaveOccurred())

		// All users must be present
		Expect(output).To(HaveKey("_meta"))
		Expect(output).To(HaveKey("admin"))
		Expect(output).To(HaveKey("kibanaserver"))
		Expect(output).To(HaveKey("logstash"))
		Expect(output).To(HaveKey("kibanaro"))

		// Verify logstash user is untouched
		logstash, ok := output["logstash"].(map[interface{}]interface{})
		Expect(ok).To(BeTrue())
		Expect(logstash["hash"]).To(Equal("$2a$12$logstashhash"))

		// Verify kibanaro user preserves attributes and multiple backend_roles
		kibanaro, ok := output["kibanaro"].(map[interface{}]interface{})
		Expect(ok).To(BeTrue())
		Expect(kibanaro["hash"]).To(Equal("$2a$12$kibanarohash"))
		Expect(kibanaro["description"]).To(Equal("kibanaro user"))
	})

	It("should create kibanaserver if it does not exist", func() {
		inputYaml := `
_meta:
  type: "internalusers"
  config_version: 2
admin:
  hash: "adminhash"
  reserved: true
  backend_roles:
    - "admin"
customuser:
  hash: "$2a$12$customhash"
  reserved: false
  description: "custom"
`
		result, err := applyUserHashes(
			[]byte(inputYaml),
			[]byte("adminpass"),
			"$2a$12$adminhash",
			[]byte("dashboardspass"),
			"$2a$12$dashhash",
		)
		Expect(err).NotTo(HaveOccurred())

		var output map[string]interface{}
		err = yaml.Unmarshal(result, &output)
		Expect(err).NotTo(HaveOccurred())

		// kibanaserver should be created
		Expect(output).To(HaveKey("kibanaserver"))
		kibana, ok := output["kibanaserver"].(map[interface{}]interface{})
		Expect(ok).To(BeTrue())
		Expect(kibana["hash"]).To(Equal("$2a$12$dashhash"))
		Expect(kibana["reserved"]).To(Equal(true))

		// Custom user must still be present
		Expect(output).To(HaveKey("customuser"))
		custom, ok := output["customuser"].(map[interface{}]interface{})
		Expect(ok).To(BeTrue())
		Expect(custom["hash"]).To(Equal("$2a$12$customhash"))
	})

	It("should add admin backend role if missing", func() {
		inputYaml := `
_meta:
  type: "internalusers"
  config_version: 2
admin:
  hash: "adminhash"
  reserved: true
  backend_roles:
    - "other_role"
kibanaserver:
  hash: "kibanahash"
  reserved: true
  description: "Demo user for the OpenSearch Dashboards server"
`
		result, err := applyUserHashes(
			[]byte(inputYaml),
			[]byte("adminpass"),
			"$2a$12$newhash",
			[]byte("dashboardspass"),
			"$2a$12$newkibanahash",
		)
		Expect(err).NotTo(HaveOccurred())

		var output map[string]interface{}
		err = yaml.Unmarshal(result, &output)
		Expect(err).NotTo(HaveOccurred())

		admin, ok := output["admin"].(map[interface{}]interface{})
		Expect(ok).To(BeTrue())

		roles, ok := admin["backend_roles"].([]interface{})
		Expect(ok).To(BeTrue())

		// Should contain both the original role and "admin"
		roleStrings := make([]string, len(roles))
		for i, r := range roles {
			roleStrings[i] = r.(string)
		}
		Expect(roleStrings).To(ContainElement("other_role"))
		Expect(roleStrings).To(ContainElement("admin"))
	})
})

// OpenSearch 2.12+ rejects admin passwords missing any of: uppercase,
// lowercase, digit, special character (see issue #1415).
func expectPolicyCompliant(password string) {
	GinkgoHelper()
	Expect(len(password)).To(BeNumerically(">=", 8))
	Expect(password).To(MatchRegexp(`[A-Z]`), "password must contain an uppercase letter")
	Expect(password).To(MatchRegexp(`[a-z]`), "password must contain a lowercase letter")
	Expect(password).To(MatchRegexp(`[0-9]`), "password must contain a digit")
	Expect(password).To(MatchRegexp(`[^A-Za-z0-9]`), "password must contain a special character")
}

var _ = Describe("GenerateSecurePassword", func() {
	It("satisfies the security plugin password policy", func() {
		for i := 0; i < 100; i++ {
			expectPolicyCompliant(GenerateSecurePassword())
		}
	})

	It("avoids shell glob characters that get mangled by unquoted expansion (issue #955)", func() {
		for i := 0; i < 100; i++ {
			Expect(GenerateSecurePassword()).ToNot(MatchRegexp(`[*?\[\]$` + "`" + `\\'"]`))
		}
	})

	It("returns unique passwords", func() {
		Expect(GenerateSecurePassword()).ToNot(Equal(GenerateSecurePassword()))
	})
})

var _ = Describe("EnsureAdminCredentialsSecret", func() {
	It("generates a password that satisfies the security plugin password policy", func() {
		cr := &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "pwtest", Namespace: "pwtest"},
		}
		secretName := GeneratedAdminCredentialsSecretName(cr)

		mockClient := k8smocks.NewMockK8sClient(GinkgoT())
		notFound := &k8serrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
		mockClient.EXPECT().GetSecret(secretName, "pwtest").Return(corev1.Secret{}, notFound).Once()
		var created *corev1.Secret
		mockClient.EXPECT().CreateSecret(mock.Anything).Run(func(secret *corev1.Secret) {
			created = secret
		}).Return(nil, nil).Once()
		mockClient.EXPECT().GetSecret(secretName, "pwtest").Return(corev1.Secret{}, nil).Once()

		_, _, err := EnsureAdminCredentialsSecret(mockClient, cr)
		Expect(err).ToNot(HaveOccurred())
		Expect(created).ToNot(BeNil())
		Expect(created.StringData["username"]).To(Equal("admin"))
		expectPolicyCompliant(created.StringData["password"])
	})
})

var _ = Describe("EnsureDashboardsCredentialsSecret", func() {
	It("generates a password that satisfies the security plugin password policy", func() {
		cr := &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "pwtest", Namespace: "pwtest"},
			Spec: opensearchv1.ClusterSpec{
				Security: &opensearchv1.Security{
					Tls: &opensearchv1.TlsConfig{
						Http: &opensearchv1.TlsConfigHttp{Enabled: ptr.To(true)},
					},
				},
			},
		}
		secretName := GeneratedDashboardsCredentialsSecretName(cr)

		mockClient := k8smocks.NewMockK8sClient(GinkgoT())
		notFound := &k8serrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
		mockClient.EXPECT().GetSecret(secretName, "pwtest").Return(corev1.Secret{}, notFound).Once()
		var created *corev1.Secret
		mockClient.EXPECT().CreateSecret(mock.Anything).Run(func(secret *corev1.Secret) {
			created = secret
		}).Return(nil, nil).Once()
		mockClient.EXPECT().GetSecret(secretName, "pwtest").Return(corev1.Secret{}, nil).Once()

		_, _, err := EnsureDashboardsCredentialsSecret(mockClient, cr)
		Expect(err).ToNot(HaveOccurred())
		Expect(created).ToNot(BeNil())
		Expect(created.StringData["username"]).To(Equal("kibanaserver"))
		expectPolicyCompliant(created.StringData["password"])
	})
})

var _ = Describe("HotReloadEnabled", func() {
	cluster := func(version string) *opensearchv1.OpenSearchCluster {
		return &opensearchv1.OpenSearchCluster{
			Spec: opensearchv1.ClusterSpec{General: opensearchv1.GeneralConfig{Version: version}},
		}
	}

	It("defaults to enabled on OpenSearch 3.x when unset", func() {
		Expect(HotReloadEnabled(cluster("3.0.0"), nil)).To(BeTrue())
	})

	It("defaults to disabled below 3.x when unset", func() {
		Expect(HotReloadEnabled(cluster("2.19.1"), nil)).To(BeFalse())
	})

	It("honors explicit true only when the version supports hot reload", func() {
		Expect(HotReloadEnabled(cluster("2.19.1"), ptr.To(true))).To(BeTrue())
		Expect(HotReloadEnabled(cluster("2.18.0"), ptr.To(true))).To(BeFalse())
	})

	It("honors explicit false on OpenSearch 3.x", func() {
		Expect(HotReloadEnabled(cluster("3.0.0"), ptr.To(false))).To(BeFalse())
	})
})

var _ = Describe("Upgrade status helpers", func() {
	Describe("ClearUpgraderComponentStatuses", func() {
		It("should remove all Upgrader entries including orphaned pools", func() {
			statuses := []opensearchv1.ComponentStatus{
				{Component: "RollingRestart", Status: "Finished"},
				{Component: "Upgrader", Description: "data", Status: "Upgraded"},
				{Component: "Upgrader", Description: "removed-pool", Status: "Upgrading"},
				{Component: "Upgrader", Description: "__upgrade_target__", Status: "2.12.0"},
				{Component: "Scaler", Description: "masters", Status: "Finished"},
			}

			result := ClearUpgraderComponentStatuses(statuses)
			Expect(result).To(ConsistOf(
				opensearchv1.ComponentStatus{Component: "RollingRestart", Status: "Finished"},
				opensearchv1.ComponentStatus{Component: "Scaler", Description: "masters", Status: "Finished"},
			))
		})
	})

	Describe("HasPinnedCustomImage", func() {
		It("should return true when a custom image is set", func() {
			image := "example.com/opensearch:1"
			cluster := &opensearchv1.OpenSearchCluster{
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						ImageSpec: &opensearchv1.ImageSpec{Image: &image},
					},
				},
			}
			Expect(HasPinnedCustomImage(cluster)).To(BeTrue())
			Expect(PinnedCustomImage(cluster)).To(Equal(image))
		})

		It("should return false when no custom image is set", func() {
			cluster := &opensearchv1.OpenSearchCluster{
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{Version: "2.12.0"},
				},
			}
			Expect(HasPinnedCustomImage(cluster)).To(BeFalse())
			Expect(PinnedCustomImage(cluster)).To(BeEmpty())
		})
	})

	Describe("IsUpgradeInProgress", func() {
		It("should be true while an upgrade target marker exists", func() {
			status := opensearchv1.ClusterStatus{
				ComponentsStatus: []opensearchv1.ComponentStatus{
					{Component: "Upgrader", Description: "__upgrade_target__", Status: "2.12.0"},
					{Component: "Upgrader", Description: "data", Status: "Upgraded"},
				},
			}
			Expect(IsUpgradeInProgress(status)).To(BeTrue())
		})

		It("should be false when only Upgraded entries remain", func() {
			status := opensearchv1.ClusterStatus{
				ComponentsStatus: []opensearchv1.ComponentStatus{
					{Component: "Upgrader", Description: "data", Status: "Upgraded"},
				},
			}
			Expect(IsUpgradeInProgress(status)).To(BeFalse())
		})
	})
})

var _ = Describe("Stuck pod handling (issue #1531)", func() {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "c-nodes", Namespace: "ns"},
		Spec:       appsv1.StatefulSetSpec{Replicas: ptr.To[int32](2)},
		Status:     appsv1.StatefulSetStatus{UpdateRevision: "rev-new"},
	}
	pod := func(name, revision, waitingReason string) corev1.Pod {
		p := corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "ns", Labels: map[string]string{stsRevisionLabel: revision},
		}}
		if waitingReason != "" {
			p.Status.ContainerStatuses = []corev1.ContainerStatus{{
				Ready: false,
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: waitingReason}},
			}}
		} else {
			p.Status.ContainerStatuses = []corev1.ContainerStatus{{Ready: true}}
		}
		return p
	}

	It("deletes an older-revision pod stuck in ImagePullBackOff", func() {
		mockClient := k8smocks.NewMockK8sClient(GinkgoT())
		mockClient.EXPECT().GetPod("c-nodes-0", "ns").Return(pod("c-nodes-0", "rev-old", "ImagePullBackOff"), nil)
		mockClient.EXPECT().DeletePod(mock.MatchedBy(func(p *corev1.Pod) bool { return p.Name == "c-nodes-0" })).Return(nil)

		deleted, err := DeleteStuckPodWithOlderRevision(mockClient, sts)
		Expect(err).ToNot(HaveOccurred())
		Expect(deleted).To(Equal("c-nodes-0"))
	})

	It("does not delete an older-revision pod that is merely starting", func() {
		mockClient := k8smocks.NewMockK8sClient(GinkgoT())
		mockClient.EXPECT().GetPod("c-nodes-0", "ns").Return(pod("c-nodes-0", "rev-old", "ContainerCreating"), nil)

		deleted, err := DeleteStuckPodWithOlderRevision(mockClient, sts)
		Expect(err).ToNot(HaveOccurred())
		Expect(deleted).To(BeEmpty())
	})

	It("reports every stuck pod with its waiting reason", func() {
		mockClient := k8smocks.NewMockK8sClient(GinkgoT())
		mockClient.EXPECT().GetPod("c-nodes-0", "ns").Return(pod("c-nodes-0", "rev-new", "ErrImagePull"), nil)
		mockClient.EXPECT().GetPod("c-nodes-1", "ns").Return(pod("c-nodes-1", "rev-new", ""), nil)

		stuck, err := StuckPods(mockClient, sts)
		Expect(err).ToNot(HaveOccurred())
		Expect(stuck).To(Equal(map[string]string{"c-nodes-0": "ErrImagePull"}))
	})
})

var _ = Describe("IsSecurityPluginEnabled and CanRunSecurityAdmin", func() {
	makeCluster := func(version string, transport *opensearchv1.TlsConfigTransport, http *opensearchv1.TlsConfigHttp) *opensearchv1.OpenSearchCluster {
		return &opensearchv1.OpenSearchCluster{
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{Version: version},
				Security: &opensearchv1.Security{
					Tls: &opensearchv1.TlsConfig{
						Transport: transport,
						Http:      http,
					},
				},
			},
		}
	}

	It("should report both disabled without any TLS", func() {
		cluster := &opensearchv1.OpenSearchCluster{
			Spec: opensearchv1.ClusterSpec{General: opensearchv1.GeneralConfig{Version: "2.19.4"}},
		}
		Expect(IsSecurityPluginEnabled(cluster)).To(BeFalse())
		Expect(CanRunSecurityAdmin(cluster)).To(BeFalse())
	})

	It("should report the plugin enabled but securityadmin unavailable with transport TLS only on >= 2.0", func() {
		cluster := makeCluster("2.19.4", &opensearchv1.TlsConfigTransport{Generate: true}, nil)
		Expect(IsSecurityPluginEnabled(cluster)).To(BeTrue())
		Expect(CanRunSecurityAdmin(cluster)).To(BeFalse())
	})

	It("should report the plugin enabled but securityadmin unavailable with HTTP TLS explicitly disabled on >= 2.0", func() {
		cluster := makeCluster(
			"2.19.4",
			&opensearchv1.TlsConfigTransport{Generate: true},
			&opensearchv1.TlsConfigHttp{Enabled: ptr.To(false)},
		)
		Expect(IsSecurityPluginEnabled(cluster)).To(BeTrue())
		Expect(CanRunSecurityAdmin(cluster)).To(BeFalse())
	})

	It("should report both enabled with transport and HTTP TLS on >= 2.0", func() {
		cluster := makeCluster(
			"2.19.4",
			&opensearchv1.TlsConfigTransport{Generate: true},
			&opensearchv1.TlsConfigHttp{Generate: true},
		)
		Expect(IsSecurityPluginEnabled(cluster)).To(BeTrue())
		Expect(CanRunSecurityAdmin(cluster)).To(BeTrue())
	})

	It("should report both enabled with transport TLS only on < 2.0 (securityadmin uses the transport port)", func() {
		cluster := makeCluster("1.3.0", &opensearchv1.TlsConfigTransport{Generate: true}, nil)
		Expect(IsSecurityPluginEnabled(cluster)).To(BeTrue())
		Expect(CanRunSecurityAdmin(cluster)).To(BeTrue())
	})
})

var _ = Describe("CountRunningPodsForNodePool", func() {
	readyPod := func(name string) corev1.Pod {
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
			Status: corev1.PodStatus{
				Conditions: []corev1.PodCondition{{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				}},
			},
		}
	}

	It("counts ready node-pool pods", func() {
		mockClient := k8smocks.NewMockK8sClient(GinkgoT())
		mockClient.EXPECT().ListPods(mock.Anything).Return(corev1.PodList{
			Items: []corev1.Pod{readyPod("cluster-master-0")},
		}, nil)

		cr := &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "ns"},
		}
		count, err := CountRunningPodsForNodePool(mockClient, cr, &opensearchv1.NodePool{Component: "master"})
		Expect(err).NotTo(HaveOccurred())
		Expect(count).To(Equal(1))
	})

})
