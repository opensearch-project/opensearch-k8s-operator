package helpers

import (
	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
