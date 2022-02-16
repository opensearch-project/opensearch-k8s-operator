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
     "settings": {
       "index": {
            "number_of_shards": 1
            }
          }
     }`)
	indexName := "cat-indices-test"
	CreateIndex(t, clusterClient, indexName, mapping)
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
	DeleteIndex(clusterClient, indexName)
}

func TestCatShards(t *testing.T) {
	clusterClient := getClusterClient(t)
	mapping := strings.NewReader(`{
     "settings": {
       "index": {
            "number_of_shards": 1,
			"number_of_replicas": 1
            }
          }
     }`)
	indexName := "cat-shards-test"
	CreateIndex(t, clusterClient, indexName, mapping)

	var headers = make([]string, 0)
	response, err := clusterClient.CatShards(headers)

	assert.Nil(t, err, "failed to cat shards")
	assert.NotEmpty(t, response, "cat shards response is empty")
	indexExists := false
	for _, res := range response {
		if indexName == res.Index {
			indexExists = true
			break
		}
	}

	assert.True(t, indexExists, "index not found")
	DeleteIndex(clusterClient, indexName)
}

func TestPutClusterSettings(t *testing.T) {
	clusterClient := getClusterClient(t)
	settingsJson := `{
 					 "transient" : {
    					"indices.recovery.max_bytes_per_sec" : "20mb"
  						}
					}`

	response, err := clusterClient.PutClusterSettings(settingsJson)

	assert.Nil(t, err, "failed to put settings")
	assert.NotEmpty(t, response.Transient, "transient settings are empty")

	response, err = clusterClient.GetClusterSettings()
	assert.Nil(t, err, "failed to put settings")
	assert.NotEmpty(t, response.Transient, "transient settings are empty")

	settingsJson = `{
 					 "transient" : {
    					"indices.recovery.max_bytes_per_sec" : null
  						}
					}`
	response, err = clusterClient.PutClusterSettings(settingsJson)
	assert.Nil(t, err, "failed to reset settings")
	indicesSettings := response.Transient["indices"]
	if indicesSettings == nil {
		assert.True(t, true, "transient settings are not empty")
	} else {
		maxBytesPerSec := indicesSettings.(map[string]map[string]interface{})
		assert.Nil(t, maxBytesPerSec["recovery"]["max_bytes_per_sec"], "transient indices settings are not empty")
	}
}
