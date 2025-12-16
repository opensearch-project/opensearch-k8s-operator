package operatortests

import (
	"time"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("DataIntegrityNodePoolOperations", func() {
	var (
		clusterName = "test-cluster"
		namespace   = "default"
		dataManager *TestDataManager
		operations  *ClusterOperations
		testData    map[string]map[string]interface{}
	)

	BeforeEach(func() {
		dataManager, operations = setupDataIntegrityTest(clusterName, namespace)
	})

	Context("Add node pool", func() {
		It("should maintain data integrity when adding a new node pool", func() {
			By("Importing test data")
			var err error
			testData, err = dataManager.ImportTestData(getDefaultTestData())
			Expect(err).NotTo(HaveOccurred())

			By("Verifying data before adding node pool")
			err = dataManager.ValidateDataIntegrity(testData)
			Expect(err).NotTo(HaveOccurred())

			By("Adding new data node pool")
			newNodePool := opsterv1.NodePool{
				Component: "data-nodes",
				Replicas:  2,
				DiskSize:  resource.MustParse("1Gi"),
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
				},
				Roles: []string{"data"},
			}
			err = operations.AddNodePool(clusterName, newNodePool)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for new node pool to be ready")
			err = operations.WaitForNodePoolReady(clusterName, "data-nodes", 2, time.Minute*10)
			Expect(err).NotTo(HaveOccurred())

			By("Reconnecting to cluster")
			err = dataManager.Reconnect()
			Expect(err).NotTo(HaveOccurred())

			By("Verifying data integrity after adding node pool")
			err = dataManager.ValidateDataIntegrity(testData)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying cluster health")
			err = dataManager.ValidateClusterHealth(true)
			Expect(err).NotTo(HaveOccurred())

			By("Removing the added node pool")
			err = operations.RemoveNodePool(clusterName, "data-nodes")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Replace node pool", func() {
		It("should maintain data integrity when adding a new data node pool and removing the old one", func() {
			// This test verifies data integrity when:
			// 1. Cluster starts with dedicated master nodes and data nodes (from manifest)
			// 2. Adding a new data node pool
			// 3. Removing the old data node pool
			// This simulates a common production scenario of replacing data nodes

			By("Importing test data")
			var err error
			testData, err = dataManager.ImportTestData(getDefaultTestData())
			Expect(err).NotTo(HaveOccurred())

			By("Verifying data before node pool operations")
			err = dataManager.ValidateDataIntegrity(testData)
			Expect(err).NotTo(HaveOccurred())

			By("Step 1: Adding new data node pool")
			newDataNodePool := opsterv1.NodePool{
				Component: "data-nodes-new",
				Replicas:  2,
				DiskSize:  resource.MustParse("1Gi"),
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
				},
				Roles: []string{"data"},
			}
			err = operations.AddNodePool(clusterName, newDataNodePool)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for new data node pool to be ready")
			err = operations.WaitForNodePoolReady(clusterName, "data-nodes-new", 2, time.Minute*10)
			Expect(err).NotTo(HaveOccurred())

			By("Reconnecting and verifying data after adding new data node pool")
			err = dataManager.Reconnect()
			Expect(err).NotTo(HaveOccurred())
			err = dataManager.ValidateDataIntegrity(testData)
			Expect(err).NotTo(HaveOccurred())
			err = dataManager.ValidateClusterHealth(true)
			Expect(err).NotTo(HaveOccurred())

			By("Step 2: Removing old data node pool")
			err = operations.RemoveNodePool(clusterName, "data")
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for old data node pool removal to process")
			time.Sleep(15 * time.Second)

			By("Reconnecting and verifying data after removing old data node pool")
			err = dataManager.Reconnect()
			Expect(err).NotTo(HaveOccurred())
			err = dataManager.ValidateDataIntegrity(testData)
			Expect(err).NotTo(HaveOccurred())
			err = dataManager.ValidateClusterHealth(true)
			Expect(err).NotTo(HaveOccurred())

			By("Final data integrity verification")
			err = dataManager.ValidateDataIntegrity(testData)
			Expect(err).NotTo(HaveOccurred())
			err = dataManager.ValidateClusterHealth(true)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
