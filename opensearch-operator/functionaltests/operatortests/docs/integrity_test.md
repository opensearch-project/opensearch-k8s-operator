# Data Integrity Tests

This document describes the data integrity test suite that verifies data integrity during various OpenSearch cluster operations. The tests are organized into separate files by operation type:

- **`upgrade_test.go`** - Tests data integrity during version upgrades
- **`scaling_test.go`** - Tests data integrity during scaling operations (scale up/down)
- **`nodepool_operations_test.go`** - Tests data integrity during node pool operations (add/remove/replace)
- **`multiple_operations_test.go`** - Tests data integrity through multiple sequential operations

## Overview

The data integrity test suite verifies that data remains intact during various cluster operations. All tests follow a common pattern:

1. **Wait for cluster to be ready** (3 master pods + 2 data pods)
2. **Connect to OpenSearch** using admin credentials
3. **Import test data**:
   - Creates 3 test indices: `test-products`, `test-orders`, `test-users`
   - Indexes multiple documents into each index
   - Verifies all data is indexed correctly
4. **Perform cluster operation** (upgrade, scale, node pool changes, etc.)
5. **Verify data integrity**:
   - Confirms all indices still exist
   - Verifies all documents are present with correct data
   - Checks cluster health status

### Test Files

- **`upgrade_test.go`** - Tests data integrity during version upgrades (2.19.4 → 3.4.0)
- **`scaling_test.go`** - Tests data integrity when scaling node pools up and down
- **`nodepool_operations_test.go`** - Tests data integrity when adding, removing, or replacing node pools
- **`multiple_operations_test.go`** - Tests data integrity through multiple sequential operations

## Test Data

The test creates realistic test data across three indices:

### test-products
- Products with fields: id, name, price, category, stock
- 3 sample products (Laptop, Mouse, Keyboard)

### test-orders
- Orders with fields: id, orderId, customerId, total, status
- 3 sample orders with different statuses

### test-users
- Users with fields: id, username, email, role, active
- 3 sample users with different roles

## Running the Test

### Prerequisites

- k3d installed and configured
- Helm installed
- Go 1.24.11 or later
- Docker (for building operator image)

### Local Execution

```bash
cd opensearch-operator/functionaltests
./execute_tests.sh
```

Or run specific test suites:

```bash
# Run upgrade tests only
cd opensearch-operator/functionaltests
go test ./operatortests -ginkgo.focus="DataIntegrityUpgrade" -timeout 30m

# Run scaling tests only
go test ./operatortests -ginkgo.focus="DataIntegrityScaling" -timeout 30m

# Run node pool operation tests only
go test ./operatortests -ginkgo.focus="DataIntegrityNodePoolOperations" -timeout 30m

# Run multiple operations tests only
go test ./operatortests -ginkgo.focus="DataIntegrityMultipleOperations" -timeout 30m

# Run all data integrity tests
go test ./operatortests -ginkgo.focus="DataIntegrity" -timeout 60m
```

### CI/CD Execution

The test will run automatically as part of the functional tests workflow in GitHub Actions when:
- A pull request is created
- The workflow is manually triggered

## Test Structure

All data integrity tests follow a common structure using shared helper functions:

### Common Setup (BeforeEach)
All tests use the `setupDataIntegrityTest()` helper function which:
- Waits for master node pool to be ready (3 replicas)
- Waits for data node pool to be ready (2 replicas)
- Initializes `TestDataManager` for data operations
- Initializes `ClusterOperations` for cluster operations

### Test Organization

#### Upgrade Tests (`upgrade_test.go`)
- **Test**: "should maintain data integrity during version upgrade"
- Imports test data
- Verifies data integrity before upgrade
- Upgrades cluster (OpenSearch and Dashboards)
- Waits for both upgrades to complete
- Reconnects and verifies data integrity after upgrade

#### Scaling Tests (`scaling_test.go`)
- **Scale Up**: Tests scaling from 2 to 4 replicas and back
- **Scale Down**: Tests scaling from 2 to 1 replica and back
- Each test verifies data integrity before and after scaling

#### Node Pool Operations Tests (`nodepool_operations_test.go`)
- **Add Node Pool**: Tests adding a new data node pool
- **Replace Node Pool**: Tests adding a new node pool and removing the old one (simulates node replacement)

#### Multiple Operations Tests (`multiple_operations_test.go`)
- Tests sequential operations: scale up → scale down → upgrade (if needed)
- Verifies data integrity after each operation

## Configuration

The test uses the `test-cluster.yaml` file which defines:
- Cluster name: `test-cluster`
- Initial version: 2.19.4
- Upgrade version: 3.4.0
- 3 master/data nodes
- 1 dashboard replica
- TLS enabled (auto-generated)

## Customization

To test different scenarios, you can modify:

1. **Test data**: Edit the `getDefaultTestData()` function in `helpers_test.go` to return different test indices and documents
2. **Upgrade versions**: Change the version parameters in `operations.UpgradeCluster()` calls
3. **Scaling values**: Modify replica counts in scaling tests
4. **Node pool configurations**: Adjust node pool specs in nodepool operations tests
5. **Cluster configuration**: Edit `test-cluster.yaml`
6. **Timeouts**: Adjust timeout values in wait functions (`WaitForUpgradeComplete()`, `WaitForNodePoolReady()`, etc.)

## Troubleshooting

### Test fails to connect to OpenSearch

- Ensure the cluster is fully ready (all pods in Running state)
- Check that credentials are correctly retrieved from the secret
- Verify the cluster URL is correct

### Data not found after upgrade

- Check cluster health status
- Verify indices were not deleted during upgrade
- Check OpenSearch logs for errors
- Ensure upgrade completed successfully

### Upgrade takes too long

- Increase timeout values in the test
- Check resource constraints (CPU/memory)
- Verify cluster is healthy before upgrade

## Extending the Tests

To add more test scenarios:

1. **Add more test data**: Extend the `getDefaultTestData()` function in `helpers_test.go`
2. **Add new test file**: Create a new `*_test.go` file following the same pattern
3. **Add new test scenarios**: Add new `Context` or `It` blocks to existing test files
4. **Test different operations**: Add search queries, aggregations, etc.
5. **Test rollback scenarios**: Add rollback verification tests
6. **Test with more data**: Increase document counts for stress testing

### Example: Adding a New Test File

```go
package operatortests

import (
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

var _ = Describe("DataIntegrityMyOperation", func() {
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

    It("should maintain data integrity during my operation", func() {
        // Import test data
        testData, err := dataManager.ImportTestData(getDefaultTestData())
        Expect(err).NotTo(HaveOccurred())

        // Perform your operation
        // ...

        // Verify data integrity
        err = dataManager.ValidateDataIntegrity(testData)
        Expect(err).NotTo(HaveOccurred())
    })
})
```

## Notes

- All tests use the `TestDataManager` and `ClusterOperations` helper classes
- The test framework uses a raw OpenSearch client for direct API access
- All test data is cleaned up after tests complete (unless `SKIP_CLEANUP` is set)
- Tests verify both index existence and document content
- Cluster health is checked to ensure the cluster is fully operational
- Tests can run independently or as part of the full suite


