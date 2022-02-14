package services

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestHasNoIndicesWithNoReplica(t *testing.T) {
	clusterClient := getClusterClient(t)
	mapping := strings.NewReader(`{
     'settings': {
       'index': {
            'number_of_shards': 1,
			'number_of_replicas': 1
            }
          }
     }`)
	indexName := "cat-indices-test"
	createIndex(t, clusterClient, indexName, mapping)
	hasNoReplicas, err := HasIndicesWithNoReplica(clusterClient)
	assert.Nil(t, err, "failed to perform HasIndicesWithNoReplica logic")
	assert.False(t, hasNoReplicas, "all indices should have replica")
	deleteIndex(clusterClient, indexName)
}

func TestHasIndicesWithNoReplica(t *testing.T) {
	clusterClient := getClusterClient(t)
	mapping := strings.NewReader(`{
     'settings': {
       'index': {
            'number_of_shards': 1,
			'number_of_replicas': 0
            }
          }
     }`)
	indexName := "cat-indices-test"
	createIndex(t, clusterClient, indexName, mapping)
	hasNoReplicas, err := HasIndicesWithNoReplica(clusterClient)
	assert.Nil(t, err, "failed to perform HasIndicesWithNoReplica logic")
	assert.True(t, hasNoReplicas, "index should have no replica")
	deleteIndex(clusterClient, indexName)
}

func TestNodeExclude(t *testing.T) {
	clusterClient := getClusterClient(t)
	nodeExcluded, err := AppendExcludeNodeHost(clusterClient, "not-exists-node")
	assert.Nil(t, err, "failed to perform AppendExcludeNodeHost logic")
	assert.True(t, nodeExcluded, "node not excluded")
	nodeExcluded, err = RemoveExcludeNodeHost(clusterClient, "not-exists-node")
	assert.Nil(t, err, "failed to perform AppendExcludeNodeHost logic")
	assert.True(t, nodeExcluded, "node not included")
}
