package operatortests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// init sets SKIP_SUITE_SETUP for operator upgrade tests so BeforeSuite skips creating test-cluster
// This runs when the package is loaded, before BeforeSuite executes
// Only sets it if not already set (allows manual override)
func init() {
	if os.Getenv("SKIP_SUITE_SETUP") == "" {
		// Set SKIP_SUITE_SETUP to prevent BeforeSuite from creating test-cluster
		// Operator upgrade tests manage their own operator and clusters
		os.Setenv("SKIP_SUITE_SETUP", "true")
	}
}

var _ = Describe("OperatorUpgrade", func() {
	var (
		clusterName     = "upgrade-test-cluster"
		namespace       = "default"
		operatorName    = "opensearch-operator"
		operatorVersion string
		clusterVersion  string
		dataManager     *TestDataManager
		operations      *ClusterOperations
		testData        map[string]map[string]interface{}
	)

	BeforeEach(func() {
		// Get the last stable version from environment variable or use default
		operatorVersion = os.Getenv("OPERATOR_STABLE_VERSION")
		if operatorVersion == "" {
			// Default to 2.8.0 as a known stable version
			operatorVersion = "2.8.0"
		}
		// Get the cluster version from environment variable or use default
		clusterVersion = os.Getenv("CLUSTER_VERSION")
		if clusterVersion == "" {
			// Default to 2.19.4
			clusterVersion = "2.19.4"
		}
	})

	AfterEach(func() {
		if !ShouldSkipCleanup() {
			// Clean up cluster
			By("Cleaning up OpenSearchCluster")
			cluster := &opensearchv1.OpenSearchCluster{}
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, cluster)
			if err == nil {
				_ = k8sClient.Delete(context.Background(), cluster)
			}

			// Clean up CRDs
			By("Cleaning up test CRDs")
			cleanupTestCRDs(clusterName, namespace)

			// Clean up NodePort service (if used)
			const nodePort int32 = 30001
			_ = CleanUpNodePort(namespace, nodePort)

			// Clean up password secret
			secret := &corev1.Secret{}
			err = k8sClient.Get(context.Background(), client.ObjectKey{Name: "test-user-password", Namespace: namespace}, secret)
			if err == nil {
				_ = k8sClient.Delete(context.Background(), secret)
			}
		}
	})

	It("should successfully upgrade operator and maintain cluster functionality", func() {
		By(fmt.Sprintf("Step 1: Installing operator version %s", operatorVersion))
		err := installOperatorFromHelm(operatorVersion)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Operator %s installed successfully\n", operatorVersion)

		By("Waiting for operator to be ready")
		// Old versions use -controller-manager suffix
		err = waitForOperatorReady(operatorName+"-controller-manager", namespace, time.Minute*5)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Operator is ready\n")

		By(fmt.Sprintf("Step 2: Creating OpenSearch cluster (version %s)", clusterVersion))
		err = createUpgradeTestCluster(clusterName, namespace, clusterVersion)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + OpenSearch cluster created\n")

		By("Initializing cluster operations helper")
		operations = NewClusterOperations(k8sClient, namespace)
		GinkgoWriter.Printf("  + Cluster operations helper initialized\n")

		By("Waiting for master node pool to be ready (3 replicas)")
		err = operations.WaitForNodePoolReady(clusterName, "masters", 3, time.Minute*15)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Master node pool ready: 3/3 replicas\n")

		By("Waiting for data node pool to be ready (3 replicas)")
		err = operations.WaitForNodePoolReady(clusterName, "data", 3, time.Minute*15)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Data node pool ready: 3/3 replicas\n")

		By("Step 3: Initializing test data manager and verifying cluster")
		dataManager, err = NewTestDataManager(k8sClient, clusterName, namespace)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Test data manager initialized\n")

		By("Importing test data into OpenSearch indices")
		testData, err = dataManager.ImportTestData(getDefaultTestData())
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Test data imported: %d indices\n", len(getDefaultTestData()))

		By("Verifying data integrity before upgrade")
		err = dataManager.ValidateDataIntegrity(testData)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Data integrity verified before upgrade\n")

		By("Verifying cluster health")
		err = dataManager.ValidateClusterHealth(true) // yellow allowed
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Cluster health verified\n")

		By("Step 4: Creating test CRDs (ActionGroup, Role, User)")
		err = createTestCRDs(clusterName, namespace)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Test CRDs created\n")

		By("Step 5: Upgrading operator to current codebase")
		err = upgradeOperatorToCurrent()
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Operator upgrade initiated\n")

		By("Waiting for operator to be ready after upgrade")
		// Current codebase doesn't use -controller-manager suffix
		err = waitForOperatorReady(operatorName, namespace, time.Minute*5)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Operator is ready after upgrade\n")

		By("Step 6: Verifying cluster is still functional after upgrade")
		// Reconnect to cluster
		err = dataManager.Reconnect()
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Reconnected to cluster\n")

		// Verify cluster health
		err = dataManager.ValidateClusterHealth(true) // yellow allowed
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Cluster health verified after upgrade\n")

		// Verify data integrity
		err = dataManager.ValidateDataIntegrity(testData)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Data integrity verified after upgrade\n")

		// Verify CRDs are still working
		By("Step 7: Verifying CRDs are still functional after upgrade")
		err = verifyTestCRDs(clusterName, namespace)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + CRDs verified after upgrade\n")

		By("Step 8: Verifying cluster can still perform operations")
		// Test that we can still perform operations on the cluster
		health, err := dataManager.osClient.GetHealth()
		Expect(err).NotTo(HaveOccurred())
		Expect(health.Status).To(BeElementOf("green", "yellow"))
		GinkgoWriter.Printf("  + Cluster operations verified: status=%s\n", health.Status)
	})
})

// installOperatorFromHelm installs the operator from Helm repo
func installOperatorFromHelm(version string) error {
	// Add helm repo if not already added
	cmd := exec.Command("helm", "repo", "add", "opensearch-operator", "https://opensearch-project.github.io/opensearch-k8s-operator/")
	_ = cmd.Run() // Ignore error if repo already exists

	// Update helm repo
	cmd = exec.Command("helm", "repo", "update")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update helm repo: %w", err)
	}

	// Use upgrade --install to handle case where operator is already installed
	cmd = exec.Command("helm", "upgrade", "--install", "opensearch-operator", "opensearch-operator/opensearch-operator",
		"--version", version,
		"--namespace", "default",
		"--wait",
		"--timeout", "5m",
		"--set", "webhook.enabled=true", // Enable webhooks for testing
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install operator: %w, output: %s", err, string(output))
	}

	return nil
}

// upgradeOperatorToCurrent upgrades the operator to the current codebase
func upgradeOperatorToCurrent() error {
	// Get the chart directory path (assuming we're in functionaltests directory)
	// Go up to opensearch-operator directory, then to charts
	chartPath := filepath.Join("..", "..", "charts", "opensearch-operator")

	// Check if chart path exists
	if _, err := os.Stat(chartPath); os.IsNotExist(err) {
		// Try alternative path (from functionaltests/operatortests)
		chartPath = filepath.Join("..", "..", "..", "charts", "opensearch-operator")
	}

	// Build the operator image first
	By("Building operator image")
	cmd := exec.Command("make", "-C", filepath.Join("..", ".."), "docker-build")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to build operator image: %w, output: %s", err, string(output))
	}

	// Import the image into k3d
	cmd = exec.Command("k3d", "image", "import", "-c", "opensearch-operator-tests", "controller:latest")
	_ = cmd.Run() // Ignore error if image already imported

	// Upgrade operator using local chart
	cmd = exec.Command("helm", "upgrade", "opensearch-operator", chartPath,
		"--namespace", "default",
		"--wait",
		"--timeout", "5m",
		"--set", "webhook.enabled=true",
		"--set", "manager.image.repository=controller",
		"--set", "manager.image.tag=latest",
		"--set", "manager.image.pullPolicy=IfNotPresent",
	)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to upgrade operator: %w, output: %s", err, string(output))
	}

	return nil
}

// waitForOperatorReady waits for the operator deployment to be ready
// It tries both possible deployment names (with and without -controller-manager suffix)
// to handle upgrades from old versions that used the suffix
func waitForOperatorReady(name, namespace string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Try both possible names (old versions use -controller-manager suffix)
	possibleNames := []string{name, name + "-controller-manager"}
	// If name already has -controller-manager, also try without it
	if strings.HasSuffix(name, "-controller-manager") {
		possibleNames = []string{name, strings.TrimSuffix(name, "-controller-manager")}
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for operator to be ready (tried: %v)", possibleNames)
		case <-ticker.C:
			for _, deploymentName := range possibleNames {
				deployment := appsv1.Deployment{}
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: deploymentName, Namespace: namespace}, &deployment)
				if err != nil {
					continue
				}

				if deployment.Status.ReadyReplicas > 0 &&
					deployment.Status.ReadyReplicas == deployment.Status.Replicas &&
					deployment.Status.UpdatedReplicas == deployment.Status.Replicas {
					return nil
				}
			}
		}
	}
}

// createUpgradeTestCluster creates a test cluster for upgrade testing
func createUpgradeTestCluster(clusterName, namespace, version string) error {
	cluster := &opensearchv1.OpenSearchCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: opensearchv1.ClusterSpec{
			General: opensearchv1.GeneralConfig{
				Version:     version,
				HttpPort:    9200,
				Vendor:      "Opensearch",
				ServiceName: clusterName,
				AdditionalConfig: map[string]string{
					"cluster.routing.allocation.disk.watermark.low":         "500m",
					"cluster.routing.allocation.disk.watermark.high":        "300m",
					"cluster.routing.allocation.disk.watermark.flood_stage": "100m",
				},
			},
			Dashboards: opensearchv1.DashboardsConfig{
				Enable:   true,
				Version:  version,
				Replicas: 1,
			},
			NodePools: []opensearchv1.NodePool{
				{
					Component: "masters",
					Replicas:  3,
					DiskSize:  resource.MustParse("1Gi"),
					Roles:     []string{"master"},
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
				},
				{
					Component: "data",
					Replicas:  3,
					DiskSize:  resource.MustParse("1Gi"),
					Roles:     []string{"data"},
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
				},
			},
			Security: &opensearchv1.Security{
				Tls: &opensearchv1.TlsConfig{
					Transport: &opensearchv1.TlsConfigTransport{
						Generate: true,
					},
					Http: &opensearchv1.TlsConfigHttp{
						Generate: true,
					},
				},
			},
		},
	}

	return k8sClient.Create(context.Background(), cluster)
}

// createTestCRDs creates test CRDs (ActionGroup, Role, User)
func createTestCRDs(clusterName, namespace string) error {
	// Create ActionGroup
	actionGroup := &unstructured.Unstructured{}
	actionGroup.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "opensearch.org",
		Version: "v1",
		Kind:    "OpensearchActionGroup",
	})
	actionGroup.SetName("test-action-group")
	actionGroup.SetNamespace(namespace)
	err := unstructured.SetNestedField(actionGroup.Object, clusterName, "spec", "opensearchCluster", "name")
	if err != nil {
		return err
	}
	err = unstructured.SetNestedStringSlice(actionGroup.Object, []string{"indices:admin/aliases/get", "indices:admin/aliases/exists"}, "spec", "allowedActions")
	if err != nil {
		return err
	}
	err = unstructured.SetNestedField(actionGroup.Object, "index", "spec", "type")
	if err != nil {
		return err
	}
	err = unstructured.SetNestedField(actionGroup.Object, "Test action group for upgrade", "spec", "description")
	if err != nil {
		return err
	}

	err = k8sClient.Create(context.Background(), actionGroup)
	if err != nil {
		return fmt.Errorf("failed to create action group: %w", err)
	}

	// Create Role
	role := &unstructured.Unstructured{}
	role.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "opensearch.org",
		Version: "v1",
		Kind:    "OpensearchRole",
	})
	role.SetName("test-role")
	role.SetNamespace(namespace)
	err = unstructured.SetNestedField(role.Object, clusterName, "spec", "opensearchCluster", "name")
	if err != nil {
		return err
	}
	// Set indexPermissions
	indexPerms := []interface{}{
		map[string]interface{}{
			"indexPatterns":  []interface{}{"test-*"},
			"allowedActions": []interface{}{"read", "write"},
		},
	}
	err = unstructured.SetNestedField(role.Object, indexPerms, "spec", "indexPermissions")
	if err != nil {
		return err
	}

	err = k8sClient.Create(context.Background(), role)
	if err != nil {
		return fmt.Errorf("failed to create role: %w", err)
	}

	// Create a secret for the user password first
	passwordSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-user-password",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"password": []byte("Test123!"),
		},
	}
	err = k8sClient.Create(context.Background(), passwordSecret)
	if err != nil {
		return fmt.Errorf("failed to create password secret: %w", err)
	}

	// Create User
	user := &unstructured.Unstructured{}
	user.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "opensearch.org",
		Version: "v1",
		Kind:    "OpensearchUser",
	})
	user.SetName("test-user")
	user.SetNamespace(namespace)
	err = unstructured.SetNestedField(user.Object, clusterName, "spec", "opensearchCluster", "name")
	if err != nil {
		return err
	}
	// Set passwordFrom
	passwordFrom := map[string]interface{}{
		"name": "test-user-password",
		"key":  "password",
	}
	err = unstructured.SetNestedField(user.Object, passwordFrom, "spec", "passwordFrom")
	if err != nil {
		return err
	}

	err = k8sClient.Create(context.Background(), user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// verifyTestCRDs verifies that the test CRDs are still present and functional
func verifyTestCRDs(clusterName, namespace string) error {
	// Verify ActionGroup
	actionGroup := &unstructured.Unstructured{}
	actionGroup.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "opensearch.org",
		Version: "v1",
		Kind:    "OpensearchActionGroup",
	})
	err := k8sClient.Get(context.Background(), client.ObjectKey{Name: "test-action-group", Namespace: namespace}, actionGroup)
	if err != nil {
		return fmt.Errorf("action group not found after upgrade: %w", err)
	}

	// Verify Role
	role := &unstructured.Unstructured{}
	role.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "opensearch.org",
		Version: "v1",
		Kind:    "OpensearchRole",
	})
	err = k8sClient.Get(context.Background(), client.ObjectKey{Name: "test-role", Namespace: namespace}, role)
	if err != nil {
		return fmt.Errorf("role not found after upgrade: %w", err)
	}

	// Verify User
	user := &unstructured.Unstructured{}
	user.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "opensearch.org",
		Version: "v1",
		Kind:    "OpensearchUser",
	})
	err = k8sClient.Get(context.Background(), client.ObjectKey{Name: "test-user", Namespace: namespace}, user)
	if err != nil {
		return fmt.Errorf("user not found after upgrade: %w", err)
	}

	return nil
}

// cleanupTestCRDs cleans up the test CRDs
func cleanupTestCRDs(clusterName, namespace string) {
	// Delete ActionGroup
	actionGroup := &unstructured.Unstructured{}
	actionGroup.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "opensearch.org",
		Version: "v1",
		Kind:    "OpensearchActionGroup",
	})
	actionGroup.SetName("test-action-group")
	actionGroup.SetNamespace(namespace)
	_ = k8sClient.Delete(context.Background(), actionGroup)

	// Delete Role
	role := &unstructured.Unstructured{}
	role.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "opensearch.org",
		Version: "v1",
		Kind:    "OpensearchRole",
	})
	role.SetName("test-role")
	role.SetNamespace(namespace)
	_ = k8sClient.Delete(context.Background(), role)

	// Delete User
	user := &unstructured.Unstructured{}
	user.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "opensearch.org",
		Version: "v1",
		Kind:    "OpensearchUser",
	})
	user.SetName("test-user")
	user.SetNamespace(namespace)
	_ = k8sClient.Delete(context.Background(), user)

	// Delete password secret
	secret := &corev1.Secret{}
	err := k8sClient.Get(context.Background(), client.ObjectKey{Name: "test-user-password", Namespace: namespace}, secret)
	if err == nil {
		_ = k8sClient.Delete(context.Background(), secret)
	}
}
