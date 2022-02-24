package services

import (
	"context"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"strings"
)

func CreateIndex(clusterClient *OsClusterClient, indexName string, mapping *strings.Reader) (int, error) {
	req := opensearchapi.IndicesCreateRequest{
		Index: indexName,
		Body:  mapping,
	}
	do, err := req.Do(context.Background(), clusterClient.client)
	return do.StatusCode, err

}

func UpdateIndexSettings(clusterClient *OsClusterClient, indexName string, mapping *strings.Reader) {
	req := opensearchapi.IndicesPutSettingsRequest{
		Index: []string{indexName},
		Body:  mapping,
	}
	_, err := req.Do(context.Background(), clusterClient.client)
	if err != nil {
		return
	}

}

func DeleteIndex(clusterClient *OsClusterClient, indexName string) (int, error) {
	res, err := opensearchapi.IndicesDeleteRequest{Index: []string{indexName}}.Do(context.Background(), clusterClient.client)
	return res.StatusCode, err

}
