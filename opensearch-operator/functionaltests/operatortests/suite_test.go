package operatortests

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Global setup/teardown for data integrity scenarios.
// This ensures the test-cluster cluster exists before any
// focused scenario (e.g. "Scale up scenario") is executed.
// For operator upgrade tests, this will be skipped if the operator isn't installed yet.

var _ = BeforeSuite(func() {
	// Check if the OpenSearchCluster CRD exists (operator is installed)
	// If not, skip creating the cluster (for operator upgrade test)
	crd := &apiextensionsv1.CustomResourceDefinition{}
	err := k8sClient.Get(context.Background(), client.ObjectKey{Name: "opensearchclusters.opensearch.opster.io"}, crd)
	if errors.IsNotFound(err) {
		By("OpenSearchCluster CRD not found - skipping test-cluster creation (operator upgrade test will install operator first)")
		return
	}
	if err != nil {
		// Some other error occurred, but we'll try to continue anyway
		By("Warning: Could not check if OpenSearchCluster CRD exists, attempting to create cluster anyway")
	}

	By("Creating OpenSearchCluster 'test-cluster'")
	// Create the OpenSearchCluster used by all data integrity tests.
	// The manifest lives under test-cluster.yaml.
	err = CreateKubernetesObjects("test-cluster")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	if !ShouldSkipCleanup() {
		By("Cleaning up NodePort service (port 30000)")
		const nodePort int32 = 30000
		_ = CleanUpNodePort("default", nodePort)

		By("Cleaning up OpenSearchCluster and related resources")
		Cleanup("test-cluster")
	} else {
		By("Skipping cleanup (SKIP_CLEANUP is set) - resources left in place for debugging")
	}
})
