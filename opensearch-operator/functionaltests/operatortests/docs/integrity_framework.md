# Data Integrity Testing Framework

This framework provides reusable components for testing data integrity during various OpenSearch cluster operations.

## Overview

The framework consists of:

1. **TestDataManager** - Handles data import and validation
2. **ClusterOperations** - Handles cluster operations (scale, upgrade, add/remove node pools)
3. **Test Scenarios** - Pre-built test scenarios using the framework

## Components

### TestDataManager

Located in `helpers_data_manager.go`, provides:

- `NewTestDataManager()` - Creates a new data manager connected to the cluster
- `Reconnect()` - Reconnects after cluster operations
- `ImportTestData()` - Imports test data into indices
- `ValidateDataIntegrity()` - Validates all data is intact
- `ValidateClusterHealth()` - Checks cluster health status
- `GetDocumentCount()` - Gets document count for an index

### ClusterOperations

Located in `helpers_cluster_operations.go`, provides:

- `UpgradeCluster()` - Upgrades cluster version
- `ScaleNodePool()` - Scales a node pool up or down
- `AddNodePool()` - Adds a new node pool
- `RemoveNodePool()` - Removes a node pool
- `WaitForNodePoolReady()` - Waits for node pool to be ready
- `WaitForUpgradeComplete()` - Waits for upgrade to complete
- `GetClusterVersion()` - Gets current cluster version
- `GetNodePoolReplicas()` - Gets current replica count

## Usage Examples

### Basic Data Import and Validation

```go
// Create data manager
dataManager, err := NewTestDataManager(k8sClient, "my-cluster", "default")
Expect(err).NotTo(HaveOccurred())

// Define test data
testIndices := []TestIndex{
    {
        Name: "my-index",
        Documents: []map[string]interface{}{
            {"id": "1", "field1": "value1"},
            {"id": "2", "field1": "value2"},
        },
    },
}

// Import data
testData, err := dataManager.ImportTestData(testIndices)
Expect(err).NotTo(HaveOccurred())

// Validate data
err = dataManager.ValidateDataIntegrity(testData)
Expect(err).NotTo(HaveOccurred())
```

### Scale Operation with Data Validation

```go
operations := NewClusterOperations(k8sClient, "default")

// Scale up
err := operations.ScaleNodePool("my-cluster", "masters", 5)
Expect(err).NotTo(HaveOccurred())

// Wait for completion
err = operations.WaitForNodePoolReady("my-cluster", "masters", 5, time.Minute*10)
Expect(err).NotTo(HaveOccurred())

// Reconnect and validate
err = dataManager.Reconnect()
Expect(err).NotTo(HaveOccurred())

err = dataManager.ValidateDataIntegrity(testData)
Expect(err).NotTo(HaveOccurred())
```

### Upgrade Operation with Data Validation

```go
// Upgrade cluster
err := operations.UpgradeCluster("my-cluster", "3.3.0", "3.3.0")
Expect(err).NotTo(HaveOccurred())

// Wait for upgrade
err = operations.WaitForUpgradeComplete("my-cluster", 
    "docker.io/opensearchproject/opensearch:3.3.0", time.Minute*15)
Expect(err).NotTo(HaveOccurred())

// Reconnect and validate
err = dataManager.Reconnect()
Expect(err).NotTo(HaveOccurred())

err = dataManager.ValidateDataIntegrity(testData)
Expect(err).NotTo(HaveOccurred())
```

### Add Node Pool with Data Validation

```go
newNodePool := opsterv1.NodePool{
    Component: "data-nodes",
    Replicas:  2,
    DiskSize:  "30Gi",
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

err := operations.AddNodePool("my-cluster", newNodePool)
Expect(err).NotTo(HaveOccurred())

err = operations.WaitForNodePoolReady("my-cluster", "data-nodes", 2, time.Minute*10)
Expect(err).NotTo(HaveOccurred())

err = dataManager.Reconnect()
Expect(err).NotTo(HaveOccurred())

err = dataManager.ValidateDataIntegrity(testData)
Expect(err).NotTo(HaveOccurred())
```

## Pre-built Test Scenarios

The test suite is organized into separate files by operation type:

### `upgrade_test.go`
- **Upgrade test** - Tests data integrity during version upgrade (2.19.4 → 3.3.0)

### `scaling_test.go`
- **Scale up** - Tests data integrity when scaling up node pools
- **Scale down** - Tests data integrity when scaling down node pools

### `nodepool_operations_test.go`
- **Add node pool** - Tests data integrity when adding a new node pool
- **Replace node pool** - Tests data integrity when adding new node pools and removing existing ones (simulates node replacement)

### `multiple_operations_test.go`
- **Multiple operations** - Tests data integrity through multiple sequential operations (scale up → scale down → upgrade)

## Creating Custom Test Scenarios

You can create custom test scenarios by combining the operations:

```go
var _ = Describe("MyCustomScenario", func() {
    var (
        clusterName = "my-cluster"
        namespace   = "default"
        dataManager *TestDataManager
        operations  *ClusterOperations
        testData    map[string]map[string]interface{}
    )

    BeforeEach(func() {
        dataManager, operations = setupDataIntegrityTest(clusterName, namespace)
    })

    It("should maintain data integrity during my operation", func() {
        // 1. Import test data
        testData, _ = dataManager.ImportTestData(getDefaultTestData())
        
        // 2. Perform your operation
        // ... your operation here ...
        
        // 3. Reconnect
        dataManager.Reconnect()
        
        // 4. Validate data integrity
        err := dataManager.ValidateDataIntegrity(testData)
        Expect(err).NotTo(HaveOccurred())
    })
})
```

## Test Data Structure

Test data is defined using `TestIndex`:

```go
type TestIndex struct {
    Name      string                          // Index name
    Settings  string                          // Optional index settings JSON
    Documents []map[string]interface{}        // Documents to index
}
```

Each document should have an `id` field (string) that will be used as the document ID.

## Best Practices

1. **Always reconnect** after cluster operations before validating data
2. **Wait for operations to complete** before validating
3. **Check cluster health** after operations
4. **Use meaningful test data** that represents your use case
5. **Clean up** added node pools or scaled resources in AfterEach if needed

## Running Tests

Run all data integrity tests:
```bash
cd opensearch-operator/functionaltests
go test ./operatortests -ginkgo.focus="DataIntegrity" -timeout 60m
```

Run specific test suites:
```bash
# Upgrade tests
go test ./operatortests -ginkgo.focus="DataIntegrityUpgrade" -timeout 30m

# Scaling tests
go test ./operatortests -ginkgo.focus="DataIntegrityScaling" -timeout 30m

# Node pool operation tests
go test ./operatortests -ginkgo.focus="DataIntegrityNodePoolOperations" -timeout 30m

# Multiple operations tests
go test ./operatortests -ginkgo.focus="DataIntegrityMultipleOperations" -timeout 30m
```

## Extending the Framework

To add new operations:

1. Add the operation method to `ClusterOperations` in `helpers_cluster_operations.go`
2. Add a corresponding wait method if needed
3. Create a new test file (e.g., `my_operation_test.go`) or add to an existing test file
4. Use the `setupDataIntegrityTest()` helper for common setup
5. Document the new operation in this README

### Shared Helper Functions

- `getDefaultTestData()` - Returns default test data (located in `helpers_test.go`)
- `setupDataIntegrityTest()` - Sets up test environment (located in `helpers_test.go`)

## Troubleshooting

### Connection Issues

If you get connection errors:
- Ensure cluster is fully ready before creating TestDataManager
- Check that credentials are correct
- Verify cluster URL is accessible

### Validation Failures

If data validation fails:
- Check cluster health status
- Verify indices weren't deleted
- Check OpenSearch logs for errors
- Ensure operations completed successfully

### Timeout Issues

If operations timeout:
- Increase timeout values
- Check resource constraints
- Verify cluster is healthy before operations


