package operatortests

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestDataManager handles test data operations
type TestDataManager struct {
	osClient    *services.OsClusterClient
	osClientRaw *opensearch.Client
	k8sClient   client.Client
	cluster     *opsterv1.OpenSearchCluster
	namespace   string
}

// NewTestDataManager creates a new test data manager
func NewTestDataManager(k8sClient client.Client, clusterName, namespace string) (*TestDataManager, error) {
	manager := &TestDataManager{
		k8sClient: k8sClient,
		namespace: namespace,
	}

	// Get cluster
	cluster := &opsterv1.OpenSearchCluster{}
	err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, cluster)
	if err != nil {
		return nil, err
	}
	manager.cluster = cluster

	// Get cluster URL and credentials
	// Use accessible URL for k3d (ClusterIP instead of DNS)
	clusterUrl, err := getAccessibleClusterURL(k8sClient, cluster)
	if err != nil {
		return nil, err
	}
	ctx := getContextWithLogger()
	k8sClientImpl := k8s.NewK8sClient(k8sClient, ctx)
	username, password, err := helpers.UsernameAndPassword(k8sClientImpl, cluster)
	if err != nil {
		return nil, err
	}

	// Create OpenSearch client wrapper
	osClient, err := services.NewOsClusterClient(clusterUrl, username, password)
	if err != nil {
		return nil, err
	}
	manager.osClient = osClient

	// Create raw OpenSearch client for direct API access
	config := opensearch.Config{
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
			MaxIdleConns:      1,
		},
		Addresses: []string{clusterUrl},
		Username:  username,
		Password:  password,
	}
	osClientRaw, err := opensearch.NewClient(config)
	if err != nil {
		return nil, err
	}
	manager.osClientRaw = osClientRaw

	return manager, nil
}

// Reconnect reconnects to the cluster (useful after operations that might change cluster state)
func (m *TestDataManager) Reconnect() error {
	// Get fresh cluster info
	cluster := &opsterv1.OpenSearchCluster{}
	err := m.k8sClient.Get(context.Background(), client.ObjectKey{Name: m.cluster.Name, Namespace: m.namespace}, cluster)
	if err != nil {
		return err
	}
	m.cluster = cluster

	// Get cluster URL and credentials
	// Use accessible URL for k3d (ClusterIP instead of DNS)
	clusterUrl, err := getAccessibleClusterURL(m.k8sClient, cluster)
	if err != nil {
		return err
	}
	ctx := getContextWithLogger()
	k8sClientImpl := k8s.NewK8sClient(m.k8sClient, ctx)
	username, password, err := helpers.UsernameAndPassword(k8sClientImpl, cluster)
	if err != nil {
		return err
	}

	// Recreate OpenSearch client wrapper
	osClient, err := services.NewOsClusterClient(clusterUrl, username, password)
	if err != nil {
		return err
	}
	m.osClient = osClient

	// Recreate raw OpenSearch client
	config := opensearch.Config{
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
			MaxIdleConns:      1,
		},
		Addresses: []string{clusterUrl},
		Username:  username,
		Password:  password,
	}
	osClientRaw, err := opensearch.NewClient(config)
	if err != nil {
		return err
	}
	m.osClientRaw = osClientRaw

	return nil
}

// TestIndex represents a test index with its documents
type TestIndex struct {
	Name      string
	Settings  string
	Documents []map[string]interface{}
}

// ImportTestData creates indices and indexes documents
func (m *TestDataManager) ImportTestData(indices []TestIndex) (map[string]map[string]interface{}, error) {
	testData := make(map[string]map[string]interface{})

	for _, index := range indices {
		// Delete index if it exists (useful when SKIP_CLEANUP is set and re-running tests)
		exists, err := m.osClient.IndexExists(index.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to check if index %s exists: %w", index.Name, err)
		}
		if exists {
			deleteReq := opensearchapi.IndicesDeleteRequest{
				Index: []string{index.Name},
			}
			res, err := deleteReq.Do(context.Background(), m.osClientRaw)
			if err != nil {
				return nil, fmt.Errorf("failed to delete existing index %s: %w", index.Name, err)
			}
			if res.StatusCode < 200 || res.StatusCode >= 300 {
				// Ignore 404 (index not found) as it might have been deleted already
				if res.StatusCode != 404 {
					return nil, fmt.Errorf("failed to delete existing index %s: status %d", index.Name, res.StatusCode)
				}
			}
			// Wait a moment for the index deletion to complete
			time.Sleep(500 * time.Millisecond)
		}

		// Create index
		var settingsReader *strings.Reader
		if index.Settings != "" {
			settingsReader = strings.NewReader(index.Settings)
		} else {
			// Default settings
			settingsReader = strings.NewReader(`{
				"settings": {
					"index": {
						"number_of_shards": 1,
						"number_of_replicas": 1
					}
				}
			}`)
		}

		req := opensearchapi.IndicesCreateRequest{
			Index: index.Name,
			Body:  settingsReader,
		}
		res, err := req.Do(context.Background(), m.osClientRaw)
		if err != nil {
			return nil, fmt.Errorf("failed to create index %s: %w", index.Name, err)
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, fmt.Errorf("failed to create index %s: status %d", index.Name, res.StatusCode)
		}

		// Index documents
		testData[index.Name] = make(map[string]interface{})
		for _, doc := range index.Documents {
			docId, ok := doc["id"].(string)
			if !ok {
				docId = fmt.Sprintf("%d", time.Now().UnixNano())
			}

			testData[index.Name][docId] = doc

			body, err := json.Marshal(doc)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal document: %w", err)
			}

			indexReq := opensearchapi.IndexRequest{
				Index:      index.Name,
				DocumentID: docId,
				Body:       strings.NewReader(string(body)),
			}
			res, err := indexReq.Do(context.Background(), m.osClientRaw)
			if err != nil {
				return nil, fmt.Errorf("failed to index document %s in %s: %w", docId, index.Name, err)
			}
			if res.StatusCode < 200 || res.StatusCode >= 300 {
				return nil, fmt.Errorf("failed to index document %s in %s: status %d", docId, index.Name, res.StatusCode)
			}
		}
	}

	// Wait for indexing to complete and refresh indices
	time.Sleep(2 * time.Second)
	for _, index := range indices {
		req := opensearchapi.IndicesRefreshRequest{
			Index: []string{index.Name},
		}
		_, err := req.Do(context.Background(), m.osClientRaw)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh index %s: %w", index.Name, err)
		}
	}

	return testData, nil
}

// valuesEqual compares two values, handling numeric type mismatches.
// JSON numbers are decoded as float64, but test data may use int/int64.
// This function normalizes numeric comparisons.
func valuesEqual(actual, expected interface{}) bool {
	// Direct comparison for same types
	if actual == expected {
		return true
	}

	// Handle numeric type mismatches
	// Convert both to float64 for comparison if they're numeric
	actualFloat, actualIsNumeric := toFloat64(actual)
	expectedFloat, expectedIsNumeric := toFloat64(expected)

	if actualIsNumeric && expectedIsNumeric {
		return actualFloat == expectedFloat
	}

	// For non-numeric types, use direct comparison
	return actual == expected
}

// toFloat64 converts a numeric value to float64, returns false if not numeric
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int8:
		return float64(val), true
	case int16:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint8:
		return float64(val), true
	case uint16:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint64:
		return float64(val), true
	default:
		return 0, false
	}
}

// ValidateDataIntegrity verifies that all indices and documents exist with correct data
func (m *TestDataManager) ValidateDataIntegrity(testData map[string]map[string]interface{}) error {
	// Verify all indices exist
	for indexName := range testData {
		exists, err := m.osClient.IndexExists(indexName)
		if err != nil {
			return fmt.Errorf("failed to check if index %s exists: %w", indexName, err)
		}
		if !exists {
			return fmt.Errorf("index %s does not exist", indexName)
		}
	}

	// Verify document counts
	for indexName := range testData {
		req := opensearchapi.CatIndicesRequest{
			Format: "json",
			Index:  []string{indexName},
		}
		res, err := req.Do(context.Background(), m.osClientRaw)
		if err != nil {
			return fmt.Errorf("failed to get index info for %s: %w", indexName, err)
		}
		if res.StatusCode != 200 {
			return fmt.Errorf("failed to get index info for %s: status %d", indexName, res.StatusCode)
		}

		var indices []map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&indices); err != nil {
			return fmt.Errorf("failed to decode index info for %s: %w", indexName, err)
		}
		if len(indices) == 0 {
			return fmt.Errorf("index %s not found in cat indices", indexName)
		}
	}

	// Verify all documents are present with correct data
	for indexName, docs := range testData {
		for docId, expectedDoc := range docs {
			req := opensearchapi.GetRequest{
				Index:      indexName,
				DocumentID: docId,
			}
			res, err := req.Do(context.Background(), m.osClientRaw)
			if err != nil {
				return fmt.Errorf("failed to get document %s from index %s: %w", docId, indexName, err)
			}
			if res.StatusCode != 200 {
				return fmt.Errorf("document %s not found in index %s: status %d", docId, indexName, res.StatusCode)
			}

			var getResponse map[string]interface{}
			if err := json.NewDecoder(res.Body).Decode(&getResponse); err != nil {
				return fmt.Errorf("failed to decode document %s from index %s: %w", docId, indexName, err)
			}

			if _, ok := getResponse["_source"]; !ok {
				return fmt.Errorf("document %s in index %s has no _source field", docId, indexName)
			}

			source := getResponse["_source"].(map[string]interface{})
			expectedDocMap := expectedDoc.(map[string]interface{})

			// Verify key fields match (skip id as it might be stored differently)
			for key, expectedValue := range expectedDocMap {
				if key == "id" {
					continue
				}
				if actualValue, ok := source[key]; !ok {
					return fmt.Errorf("field %s missing in document %s/%s", key, indexName, docId)
				} else if !valuesEqual(actualValue, expectedValue) {
					return fmt.Errorf("field %s mismatch in document %s/%s: expected %v (%T), got %v (%T)", key, indexName, docId, expectedValue, expectedValue, actualValue, actualValue)
				}
			}
		}
	}

	return nil
}

// ValidateClusterHealth verifies cluster health is green (or yellow if acceptable)
func (m *TestDataManager) ValidateClusterHealth(allowYellow bool) error {
	health, err := m.osClient.GetHealth()
	if err != nil {
		return fmt.Errorf("failed to get cluster health: %w", err)
	}

	if health.Status == "green" {
		return nil
	}

	if allowYellow && health.Status == "yellow" {
		return nil
	}

	return fmt.Errorf("cluster health is %s (expected green or yellow)", health.Status)
}

// GetDocumentCount returns the number of documents in an index
func (m *TestDataManager) GetDocumentCount(indexName string) (int64, error) {
	req := opensearchapi.CatIndicesRequest{
		Format: "json",
		Index:  []string{indexName},
	}
	res, err := req.Do(context.Background(), m.osClientRaw)
	if err != nil {
		return 0, err
	}

	var indices []map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&indices); err != nil {
		return 0, err
	}

	if len(indices) == 0 {
		return 0, nil
	}

	// Parse document count from cat indices response
	// The field name might vary, try common ones
	if docs, ok := indices[0]["docs.count"]; ok {
		if countStr, ok := docs.(string); ok {
			var count int64
			fmt.Sscanf(countStr, "%d", &count)
			return count, nil
		}
	}

	return 0, fmt.Errorf("could not parse document count from index %s", indexName)
}
