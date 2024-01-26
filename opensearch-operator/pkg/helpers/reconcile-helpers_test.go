package helpers

import (
	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = DescribeTable("versionCheck reconciler",
	func(version string, specifiedHttpPort int32, expectedHttpPort int32, expectedSecurityConfigPort int32, expectedSecurityConfigPath string) {
		instance := &opsterv1.OpenSearchCluster{
			Spec: opsterv1.ClusterSpec{
				General: opsterv1.GeneralConfig{
					Version:  version,
					HttpPort: specifiedHttpPort,
				},
			},
		}

		actualHttpPort, actualSecurityConfigPort, actualConfigPath := VersionCheck(instance)

		Expect(actualHttpPort).To(Equal(expectedHttpPort))
		Expect(actualSecurityConfigPort).To(Equal(expectedSecurityConfigPort))
		Expect(actualConfigPath).To(Equal(expectedSecurityConfigPath))
	},
	Entry("When no http port is specified and version 1.3.0 is used", "1.3.0", int32(0), int32(9200), int32(9300), "/usr/share/opensearch/plugins/opensearch-security/securityconfig"),
	Entry("When no http port is specified and version 2.0 is used", "2.0", int32(0), int32(9200), int32(9200), "/usr/share/opensearch/config/opensearch-security"),
	Entry("When an http port is specified and version 1.3.0 is used", "1.3.0", int32(6000), int32(6000), int32(9300), "/usr/share/opensearch/plugins/opensearch-security/securityconfig"),
	Entry("When an http port is specified and version 2.0 is used", "2.0", int32(6000), int32(6000), int32(6000), "/usr/share/opensearch/config/opensearch-security"),
)
