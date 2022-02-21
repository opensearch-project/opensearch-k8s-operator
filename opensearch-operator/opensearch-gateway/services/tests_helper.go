package services

import (
	"context"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"strings"
)

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
