package operatortests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	opsterv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Note: Migration tests manage their own operator and clusters.
// Set SKIP_SUITE_SETUP=true environment variable when running these tests
// to prevent the shared BeforeSuite from creating test-cluster.
// Example: SKIP_SUITE_SETUP=true ginkgo --focus "APIGroupMigration"

var _ = Describe("APIGroupMigration", func() {
	var (
		clusterName     = "migration-test-cluster"
		namespace       = "default"
		operatorName    = "opensearch-operator"
		operatorVersion string
		clusterVersion  string
		operations      *ClusterOperations
	)

	BeforeEach(func() {
		// Get the last stable version from environment variable or use default
		operatorVersion = os.Getenv("OPERATOR_STABLE_VERSION")
		if operatorVersion == "" {
			// Default to 2.8.0 as a known stable version that uses opensearch.opster.io/v1
			operatorVersion = "2.8.0"
		}
		// Get the cluster version from environment variable or use default
		clusterVersion = os.Getenv("CLUSTER_VERSION")
		if clusterVersion == "" {
			// Default to 2.19.4
			clusterVersion = "2.19.4"
		}

		// Ensure clean state before each test
		cleanupMigrationTestResources(clusterName, namespace)
	})

	AfterEach(func() {
		if !ShouldSkipCleanup() {
			cleanupMigrationTestResources(clusterName, namespace)
		}
	})

	It("should automatically migrate resources from opensearch.opster.io/v1 to opensearch.org/v1", func() {
		By(fmt.Sprintf("Step 1: Installing operator version %s (uses opensearch.opster.io/v1)", operatorVersion))
		err := installOperatorFromHelm(operatorVersion)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Operator %s installed successfully\n", operatorVersion)

		By("Waiting for operator to be ready")
		err = waitForOperatorReady(operatorName+"-controller-manager", namespace, time.Minute*5)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Operator is ready\n")

		By(fmt.Sprintf("Step 2: Creating OpenSearch cluster with old API group (opensearch.opster.io/v1, version %s)", clusterVersion))
		err = createOldAPIGroupCluster(clusterName, namespace, clusterVersion)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + OpenSearch cluster created with old API group\n")

		By("Verifying old API group cluster exists")
		// If we can retrieve it as opsterv1.OpenSearchCluster, it means the API group matches
		oldCluster := &opsterv1.OpenSearchCluster{}
		err = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, oldCluster)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Old API group cluster verified (opensearch.opster.io/v1)\n")

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

		By("Waiting for cluster to reach RUNNING phase (required for migration)")
		err = waitForClusterPhase(clusterName, namespace, "RUNNING", time.Minute*10)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Cluster is in RUNNING phase\n")

		By("Step 3: Creating test CRDs with old API group")
		err = createOldAPIGroupTestCRDs(clusterName, namespace)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Test CRDs created with old API group\n")

		By("Waiting for all old CRDs to be ready before upgrading operator")
		err = waitForOldAPIGroupCRDsReady(namespace, time.Minute*5)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + All old CRDs are ready\n")

		By("Step 4: Upgrading operator to current codebase (uses opensearch.org/v1)")
		err = upgradeOperatorToCurrent()
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Operator upgrade initiated\n")

		By("Waiting for operator to be ready after upgrade")
		err = waitForOperatorReady(operatorName, namespace, time.Minute*5)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Operator is ready after upgrade\n")

		By("Step 5: Verifying migration controller created new API group resources")
		// Wait for migration to complete
		err = waitForNewAPIGroupResource(clusterName, namespace, "opensearchclusters", "opensearch.org", time.Minute*5)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + New API group cluster resource created\n")

		// Verify new cluster exists
		// If we can retrieve it as opensearchv1.OpenSearchCluster, it means the API group matches
		newCluster := &opensearchv1.OpenSearchCluster{}
		err = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, newCluster)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + New API group cluster verified (opensearch.org/v1)\n")

		// Verify migration annotations
		Expect(newCluster.Annotations).To(HaveKey("opensearch.org/migrated-from"))
		Expect(newCluster.Annotations["opensearch.org/migrated-from"]).To(Equal("opensearch.opster.io/v1"))
		GinkgoWriter.Printf("  + Migration annotations verified\n")

		By("Step 6: Verifying both old and new resources exist")
		// Old resource should still exist
		err = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, oldCluster)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Old API group resource still exists\n")

		// New resource should exist
		err = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, newCluster)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + New API group resource exists\n")

		By("Step 7: Verifying status synchronization")
		// Wait a bit for status sync
		time.Sleep(10 * time.Second)

		// Get both resources again to check status
		err = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, oldCluster)
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, newCluster)
		Expect(err).NotTo(HaveOccurred())

		// Status should be synced (both should have the same phase)
		Expect(newCluster.Status.Phase).To(Equal(oldCluster.Status.Phase))
		GinkgoWriter.Printf("  + Status synchronized: phase=%s\n", newCluster.Status.Phase)

		By("Step 8: Verifying test CRDs were migrated")
		// Wait for CRD migration
		err = waitForNewAPIGroupResource("migration-test-action-group", namespace, "opensearchactiongroups", "opensearch.org", time.Minute*2)
		Expect(err).NotTo(HaveOccurred())

		err = waitForNewAPIGroupResource("migration-test-role", namespace, "opensearchroles", "opensearch.org", time.Minute*2)
		Expect(err).NotTo(HaveOccurred())

		err = waitForNewAPIGroupResource("migration-test-user", namespace, "opensearchusers", "opensearch.org", time.Minute*2)
		Expect(err).NotTo(HaveOccurred())

		GinkgoWriter.Printf("  + All test CRDs migrated successfully\n")

		By("Step 9: Testing deletion behavior - deleting new resource should delete old resource")
		// Delete new cluster
		err = k8sClient.Delete(context.Background(), newCluster)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + New API group cluster deletion initiated\n")

		// Wait for old cluster to be deleted
		err = waitForResourceDeletion(clusterName, namespace, "opensearchclusters", "opensearch.opster.io", time.Minute*2)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("  + Old API group cluster deleted (as expected when new is deleted)\n")

		// Verify old cluster is gone
		err = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, oldCluster)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))
		GinkgoWriter.Printf("  + Deletion behavior verified: new deletion triggers old deletion\n")
	})

	//	It("should prevent deletion of old resource if new resource does not exist", func() {
	//		By(fmt.Sprintf("Step 1: Installing operator version %s", operatorVersion))
	//		err := installOperatorFromHelm(operatorVersion)
	//		Expect(err).NotTo(HaveOccurred())
	//
	//		By("Waiting for operator to be ready")
	//		err = waitForOperatorReady(operatorName+"-controller-manager", namespace, time.Minute*5)
	//		Expect(err).NotTo(HaveOccurred())
	//
	//		By(fmt.Sprintf("Step 2: Creating OpenSearch cluster with old API group (version %s)", clusterVersion))
	//		err = createOldAPIGroupCluster(clusterName+"-deletion-test", namespace, clusterVersion)
	//		Expect(err).NotTo(HaveOccurred())
	//
	//		By("Waiting for cluster to be ready")
	//		err = waitForClusterPhase(clusterName+"-deletion-test", namespace, "RUNNING", time.Minute*15)
	//		Expect(err).NotTo(HaveOccurred())
	//
	//		By("Step 3: Upgrading operator to current codebase")
	//		err = upgradeOperatorToCurrent()
	//		Expect(err).NotTo(HaveOccurred())
	//
	//		By("Waiting for operator to be ready after upgrade")
	//		err = waitForOperatorReady(operatorName, namespace, time.Minute*5)
	//		Expect(err).NotTo(HaveOccurred())
	//
	//		By("Step 4: Attempting to delete old resource before migration completes")
	//		// Don't wait for migration - try to delete immediately
	//		oldCluster := &opsterv1.OpenSearchCluster{}
	//		err = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-deletion-test", Namespace: namespace}, oldCluster)
	//		Expect(err).NotTo(HaveOccurred())
	//
	//		// Add finalizer manually to simulate migration controller behavior
	//		if !containsString(oldCluster.Finalizers, "opensearch.org/migration") {
	//			oldCluster.Finalizers = append(oldCluster.Finalizers, "migration.opensearch.org/finalizer")
	//			err = k8sClient.Update(context.Background(), oldCluster)
	//			Expect(err).NotTo(HaveOccurred())
	//		}
	//
	//		// Try to delete
	//		err = k8sClient.Delete(context.Background(), oldCluster)
	//		Expect(err).NotTo(HaveOccurred())
	//
	//		// Wait a bit - deletion should be blocked by finalizer
	//		time.Sleep(5 * time.Second)
	//
	//		// Resource should still exist (finalizer prevents deletion)
	//		err = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-deletion-test", Namespace: namespace}, oldCluster)
	//		Expect(err).NotTo(HaveOccurred())
	//		Expect(oldCluster.DeletionTimestamp).NotTo(BeNil())
	//		GinkgoWriter.Printf("  + Old resource deletion is blocked (finalizer present, new resource not found)\n")
	//
	//		// Now wait for migration to complete
	//		err = waitForNewAPIGroupResource(clusterName+"-deletion-test", namespace, "opensearchclusters", "opensearch.org", time.Minute*5)
	//		Expect(err).NotTo(HaveOccurred())
	//
	//		// Once new resource exists, old resource should be deletable
	//		// The migration controller should remove the finalizer and allow deletion
	//		// Wait a bit for the controller to process
	//		time.Sleep(10 * time.Second)
	//
	//		// Check if old resource is still there (it should be gone or going)
	//		err = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-deletion-test", Namespace: namespace}, oldCluster)
	//		// It might be deleted or still have finalizer - both are acceptable
	//		// The key is that once new resource exists, deletion can proceed
	//		GinkgoWriter.Printf("  + Deletion behavior verified: old resource deletion requires new resource to exist\n")
	//	})
})

// createOldAPIGroupCluster creates a cluster using the old API group (opensearch.opster.io/v1)
func createOldAPIGroupCluster(clusterName, namespace, version string) error {
	cluster := &opsterv1.OpenSearchCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
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
			Dashboards: opsterv1.DashboardsConfig{
				Enable:   true,
				Version:  version,
				Replicas: 1,
			},
			NodePools: []opsterv1.NodePool{
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
			Security: &opsterv1.Security{
				Tls: &opsterv1.TlsConfig{
					Transport: &opsterv1.TlsConfigTransport{
						Generate: true,
					},
					Http: &opsterv1.TlsConfigHttp{
						Generate: true,
					},
				},
			},
		},
	}

	return k8sClient.Create(context.Background(), cluster)
}

// createOldAPIGroupTestCRDs creates test CRDs using the old API group
func createOldAPIGroupTestCRDs(clusterName, namespace string) error {
	// Create ActionGroup
	actionGroup := &unstructured.Unstructured{}
	actionGroup.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "opensearch.opster.io",
		Version: "v1",
		Kind:    "OpensearchActionGroup",
	})
	actionGroup.SetName("migration-test-action-group")
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
	err = unstructured.SetNestedField(actionGroup.Object, "Test action group for migration", "spec", "description")
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
		Group:   "opensearch.opster.io",
		Version: "v1",
		Kind:    "OpensearchRole",
	})
	role.SetName("migration-test-role")
	role.SetNamespace(namespace)
	err = unstructured.SetNestedField(role.Object, clusterName, "spec", "opensearchCluster", "name")
	if err != nil {
		return err
	}
	indexPerms := []interface{}{
		map[string]interface{}{
			"indexPatterns":  []interface{}{"migration-test-*"},
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
			Name:      "migration-test-user-password",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"password": []byte("MigrationTest123!"),
		},
	}
	err = k8sClient.Create(context.Background(), passwordSecret)
	if err != nil {
		return fmt.Errorf("failed to create password secret: %w", err)
	}

	// Create User
	user := &unstructured.Unstructured{}
	user.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "opensearch.opster.io",
		Version: "v1",
		Kind:    "OpensearchUser",
	})
	user.SetName("migration-test-user")
	user.SetNamespace(namespace)
	err = unstructured.SetNestedField(user.Object, clusterName, "spec", "opensearchCluster", "name")
	if err != nil {
		return err
	}
	passwordFrom := map[string]interface{}{
		"name": "migration-test-user-password",
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

// cleanupMigrationTestCRDs cleans up test CRDs from both API groups
func cleanupMigrationTestCRDs(clusterName, namespace string) {
	// Clean up new API group CRDs
	cleanupNewAPIGroupCRD("migration-test-action-group", namespace, "opensearchactiongroups", "opensearch.org")
	cleanupNewAPIGroupCRD("migration-test-role", namespace, "opensearchroles", "opensearch.org")
	cleanupNewAPIGroupCRD("migration-test-user", namespace, "opensearchusers", "opensearch.org")

	// Clean up old API group CRDs
	cleanupOldAPIGroupCRD("migration-test-action-group", namespace, "opensearchactiongroups", "opensearch.opster.io")
	cleanupOldAPIGroupCRD("migration-test-role", namespace, "opensearchroles", "opensearch.opster.io")
	cleanupOldAPIGroupCRD("migration-test-user", namespace, "opensearchusers", "opensearch.opster.io")
}

// kindNameMap maps plural resource names to their Kind names
var kindNameMap = map[string]string{
	"opensearchclusters":           "OpenSearchCluster",
	"opensearchactiongroups":       "OpensearchActionGroup",
	"opensearchroles":              "OpensearchRole",
	"opensearchusers":              "OpensearchUser",
	"opensearchuserrolebindings":   "OpensearchUserRoleBinding",
	"opensearchtenants":            "OpensearchTenant",
	"opensearchismpolicies":        "OpensearchISMPolicy",
	"opensearchsnapshotpolicies":   "OpensearchSnapshotPolicy",
	"opensearchindextemplates":     "OpensearchIndexTemplate",
	"opensearchcomponenttemplates": "OpensearchComponentTemplate",
}

func getKindName(plural string) string {
	if kind, ok := kindNameMap[strings.ToLower(plural)]; ok {
		return kind
	}
	// Fallback: capitalize first letter and remove trailing 's'
	if len(plural) > 0 {
		return strings.ToUpper(plural[:1]) + plural[1:len(plural)-1]
	}
	return plural
}

func cleanupNewAPIGroupCRD(name, namespace, kind, group string) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: "v1",
		Kind:    getKindName(kind),
	})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	_ = k8sClient.Delete(context.Background(), obj)
}

func cleanupOldAPIGroupCRD(name, namespace, kind, group string) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: "v1",
		Kind:    getKindName(kind),
	})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	_ = k8sClient.Delete(context.Background(), obj)
}

// waitForClusterPhase waits for a cluster to reach a specific phase
func waitForClusterPhase(clusterName, namespace, phase string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for cluster %s to reach phase %s", clusterName, phase)
		case <-ticker.C:
			// Try old API group first
			oldCluster := &opsterv1.OpenSearchCluster{}
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, oldCluster)
			if err == nil && oldCluster.Status.Phase == phase {
				return nil
			}

			// Try new API group
			newCluster := &opensearchv1.OpenSearchCluster{}
			err = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, newCluster)
			if err == nil && newCluster.Status.Phase == phase {
				return nil
			}
		}
	}
}

// waitForOldAPIGroupCRDsReady waits for all old API group CRDs to be ready before upgrading operator
func waitForOldAPIGroupCRDsReady(namespace string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// List of CRDs to check: name, kind, expected state
	crdsToCheck := []struct {
		name          string
		kind          string
		expectedState string
	}{
		{"migration-test-action-group", "OpensearchActionGroup", "CREATED"},
		{"migration-test-role", "OpensearchRole", "CREATED"},
		{"migration-test-user", "OpensearchUser", "CREATED"},
	}

	for {
		select {
		case <-ctx.Done():
			// On timeout, check which CRDs are not ready for better error message
			var notReady []string
			for _, crd := range crdsToCheck {
				obj := &unstructured.Unstructured{}
				obj.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "opensearch.opster.io",
					Version: "v1",
					Kind:    crd.kind,
				})
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: crd.name, Namespace: namespace}, obj)
				if err != nil {
					notReady = append(notReady, fmt.Sprintf("%s (not found)", crd.name))
					continue
				}

				state, found, _ := unstructured.NestedString(obj.Object, "status", "state")
				if !found {
					notReady = append(notReady, fmt.Sprintf("%s (no status)", crd.name))
				} else if state == "ERROR" {
					reason, _, _ := unstructured.NestedString(obj.Object, "status", "reason")
					notReady = append(notReady, fmt.Sprintf("%s (ERROR: %s)", crd.name, reason))
				} else if state != crd.expectedState {
					notReady = append(notReady, fmt.Sprintf("%s (state: %s, expected: %s)", crd.name, state, crd.expectedState))
				}
			}
			return fmt.Errorf("timeout waiting for old API group CRDs to be ready. Not ready: %v", notReady)
		case <-ticker.C:
			allReady := true
			for _, crd := range crdsToCheck {
				obj := &unstructured.Unstructured{}
				obj.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "opensearch.opster.io",
					Version: "v1",
					Kind:    crd.kind,
				})
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: crd.name, Namespace: namespace}, obj)
				if err != nil {
					allReady = false
					continue
				}

				// Check status.state field
				state, found, err := unstructured.NestedString(obj.Object, "status", "state")
				if err != nil || !found {
					allReady = false
					continue
				}

				// Fail early if any resource is in ERROR state
				if state == "ERROR" {
					reason, _, _ := unstructured.NestedString(obj.Object, "status", "reason")
					return fmt.Errorf("CRD %s/%s is in ERROR state: %s", crd.kind, crd.name, reason)
				}

				// Accept CREATED or IGNORED as ready states
				if state != crd.expectedState && state != "IGNORED" {
					allReady = false
					continue
				}
			}

			if allReady {
				return nil
			}
		}
	}
}

// waitForNewAPIGroupResource waits for a resource with the new API group to be created
func waitForNewAPIGroupResource(name, namespace, kind, group string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for %s/%s in group %s to be created", kind, name, group)
		case <-ticker.C:
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   group,
				Version: "v1",
				Kind:    getKindName(kind),
			})
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name, Namespace: namespace}, obj)
			if err == nil {
				return nil
			}
		}
	}
}

// waitForResourceDeletion waits for a resource to be deleted
// It returns success if:
// 1. The resource is not found (IsNotFound error) - resource was deleted
// 2. The resource type doesn't exist (CRD not installed) - resource can't exist, so treat as deleted
func waitForResourceDeletion(name, namespace, kind, group string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for %s/%s in group %s to be deleted", kind, name, group)
		case <-ticker.C:
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   group,
				Version: "v1",
				Kind:    getKindName(kind),
			})
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name, Namespace: namespace}, obj)
			if err != nil {
				// Resource not found - successfully deleted
				if apierrors.IsNotFound(err) {
					return nil
				}
				// Check if error indicates the resource type doesn't exist (CRD not installed)
				// This happens when the operator is not installed
				errStr := err.Error()
				if strings.Contains(errStr, "doesn't have a resource type") ||
					strings.Contains(errStr, "no matches for kind") ||
					strings.Contains(errStr, "the server could not find the requested resource") {
					// CRD doesn't exist, so the resource can't exist - treat as deleted
					return nil
				}
				// Other errors - continue waiting
			}
		}
	}
}

// containsString checks if a string slice contains a specific string
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// cleanupMigrationTestResources cleans up all resources created by migration tests
func cleanupMigrationTestResources(clusterName, namespace string) {
	deletionTestClusterName := clusterName + "-deletion-test"
	clusterNames := []string{clusterName, deletionTestClusterName}

	// Step 1: Delete all CRs first (before uninstalling operator, so finalizers can be processed)
	By("Deleting OpenSearchCluster resources")
	for _, cn := range clusterNames {
		// Delete new API group cluster
		newCluster := &opensearchv1.OpenSearchCluster{}
		err := k8sClient.Get(context.Background(), client.ObjectKey{Name: cn, Namespace: namespace}, newCluster)
		if err == nil {
			_ = k8sClient.Delete(context.Background(), newCluster)
		}

		// Delete old API group cluster
		oldCluster := &opsterv1.OpenSearchCluster{}
		err = k8sClient.Get(context.Background(), client.ObjectKey{Name: cn, Namespace: namespace}, oldCluster)
		if err == nil {
			_ = k8sClient.Delete(context.Background(), oldCluster)
		}
	}

	// Step 2: Wait for clusters to be fully deleted (not just terminating)
	By("Waiting for OpenSearchCluster resources to be fully deleted")
	for _, cn := range clusterNames {
		// Wait for new API group cluster deletion
		_ = waitForResourceDeletion(cn, namespace, "opensearchclusters", "opensearch.org", time.Minute*2)
		// Wait for old API group cluster deletion
		_ = waitForResourceDeletion(cn, namespace, "opensearchclusters", "opensearch.opster.io", time.Minute*2)
	}

	// Step 3: Clean up test CRDs (both old and new API groups)
	By("Cleaning up test CRDs")
	cleanupMigrationTestCRDs(clusterName, namespace)

	// Step 4: Clean up other resources
	// Clean up NodePort service (if used)
	const nodePort int32 = 30002
	_ = CleanUpNodePort(namespace, nodePort)

	// Clean up password secret
	secret := &corev1.Secret{}
	err := k8sClient.Get(context.Background(), client.ObjectKey{Name: "migration-test-user-password", Namespace: namespace}, secret)
	if err == nil {
		_ = k8sClient.Delete(context.Background(), secret)
	}

	// Clean up PVCs for both cluster names (main test and deletion test)
	By("Cleaning up PVCs")
	pvcList := &corev1.PersistentVolumeClaimList{}
	err = k8sClient.List(context.Background(), pvcList, client.InNamespace(namespace))
	if err == nil {
		for _, pvc := range pvcList.Items {
			shouldDelete := false
			// Check if PVC belongs to any of our test clusters
			for _, cn := range clusterNames {
				// Check new API group label
				if pvc.Labels["opensearch.org/opensearch-cluster"] == cn {
					shouldDelete = true
					break
				}
				// Check old API group label (if it exists)
				if pvc.Labels["opster.io/opensearch-cluster"] == cn {
					shouldDelete = true
					break
				}
				// Check if name starts with cluster name (for bootstrap PVCs: {cluster-name}-bootstrap-data)
				// or StatefulSet PVCs: data-{cluster-name}-{component}-{ordinal}
				if strings.HasPrefix(pvc.Name, cn+"-") || strings.Contains(pvc.Name, "-"+cn+"-") {
					shouldDelete = true
					break
				}
			}
			if shouldDelete {
				_ = k8sClient.Delete(context.Background(), &pvc)
			}
		}
	}

	// Step 5: Uninstall operator only after CRs are fully deleted
	By("Uninstalling operator (after CRs are deleted)")
	cmd := exec.Command("helm", "uninstall", "opensearch-operator", "--namespace", "default")
	_ = cmd.Run() // Ignore error if not installed
	// Wait a bit for cleanup
	time.Sleep(2 * time.Second)
}
