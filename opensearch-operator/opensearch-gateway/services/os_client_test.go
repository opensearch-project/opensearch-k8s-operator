package services

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestCatNodes(t *testing.T) {
	clusterClient := getClusterClient(t)
	response, err := clusterClient.CatNodes()
	assert.Nil(t, err, "failed to cat nodes")
	assert.NotEmpty(t, response, "cat nodes response is empty")
	assert.NotEmpty(t, response.Ip, "cat nodes response Ip is empty")
}

func TestNodesStats(t *testing.T) {
	clusterClient := getClusterClient(t)
	response, err := clusterClient.NodesStats()
	assert.Nil(t, err, "failed to nodes stats")
	assert.NotEmpty(t, response.Nodes, "nodes stats nodes are empty")
}

func TestCatIndices(t *testing.T) {
	clusterClient := getClusterClient(t)
	mapping := strings.NewReader(`{
     'settings': {
       'index': {
            'number_of_shards': 1
            }
          }
     }`)
	indexName := "cat-indices-test"
	createIndex(t, clusterClient, indexName, mapping)
	response, err := clusterClient.CatIndices()

	assert.Nil(t, err, "failed to indices")
	assert.NotEmpty(t, response, "cat indices response is empty")
	indexExists := false
	for _, res := range response {
		if indexName == res.Index {
			indexExists = true
			break
		}
	}

	assert.True(t, indexExists, "index not found")
	deleteIndex(clusterClient, indexName)
}
