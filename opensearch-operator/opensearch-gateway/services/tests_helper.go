package services

import (
	"context"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"strings"
)

/*func getClusterClient(t *testing.T) *OsClusterClient {
	// Initialize the client with SSL/TLS enabled.
	config := opensearch.Config{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Addresses: []string{"https://localhost:9200"},
		Username:  "admin", // For testing only. Don't store credentials in code.
		Password:  "admin",
	}

	clusterClient, err := NewOsClusterClient(config)
	assert.Nil(t, err, "failed connection to cluster")
	return clusterClient
}*/

func CreateIndex(clusterClient *OsClusterClient, indexName string, mapping *strings.Reader) {
	req := opensearchapi.IndicesCreateRequest{
		Index: indexName,
		Body:  mapping,
	}
	req.Do(context.Background(), clusterClient.client)

}

func UpdateIndexSettings(clusterClient *OsClusterClient, indexName string, mapping *strings.Reader) {
	req := opensearchapi.IndicesPutSettingsRequest{
		Index: []string{indexName},
		Body:  mapping,
	}
	req.Do(context.Background(), clusterClient.client)

}

func DeleteIndex(clusterClient *OsClusterClient, indexName string) {
	opensearchapi.IndicesDeleteRequest{Index: []string{indexName}}.Do(context.Background(), clusterClient.client)

}
