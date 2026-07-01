package helpers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"k8s.io/utils/ptr"
)

var _ = DescribeTable("versionCheck reconciler",
	func(version string, specifiedHttpPort int32, expectedHttpPort int32, expectedSecurityConfigPort int32, expectedSecurityConfigPath string) {
		instance := &opensearchv1.OpenSearchCluster{
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{
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
	Entry("When no http port is specified and prerelease version 3.0.0-testing is used", "3.0.0-testing", int32(0), int32(9200), int32(9200), "/usr/share/opensearch/config/opensearch-security"),
	Entry("When no http port is specified and prerelease version 2.0.0-testing is used", "2.0.0-testing", int32(0), int32(9200), int32(9200), "/usr/share/opensearch/config/opensearch-security"),
	Entry("When no http port is specified and prerelease version 1.9.0-testing is used", "1.9.0-testing", int32(0), int32(9200), int32(9300), "/usr/share/opensearch/plugins/opensearch-security/securityconfig"),
)

var _ = DescribeTable("ResolveImage",
	func(generalImage, nodePoolImage *string, expectedImage string) {
		cluster := &opensearchv1.OpenSearchCluster{
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{
					Version: "2.17.1",
				},
			},
		}
		if generalImage != nil {
			cluster.Spec.General.ImageSpec = &opensearchv1.ImageSpec{Image: generalImage}
		}

		var nodePool *opensearchv1.NodePool
		if nodePoolImage != nil {
			nodePool = &opensearchv1.NodePool{
				ImageSpec: &opensearchv1.ImageSpec{Image: nodePoolImage},
			}
		}

		result := ResolveImage(cluster, nodePool)
		Expect(result.GetImage()).To(Equal(expectedImage))
	},
	Entry("uses default image when no overrides are set", nil, nil, "docker.io/opensearchproject/opensearch:2.17.1"),
	Entry("uses general custom image when configured", ptr.To("custom/opensearch:1.0.0"), nil, "custom/opensearch:1.0.0"),
	Entry("uses node pool image over general image", ptr.To("custom/opensearch:1.0.0"), ptr.To("custom/cuda-opensearch:1.0.0"), "custom/cuda-opensearch:1.0.0"),
	Entry("uses node pool image when only node pool is configured", nil, ptr.To("custom/cuda-opensearch:2.17.1"), "custom/cuda-opensearch:2.17.1"),
)
