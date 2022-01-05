package services

import (
	"context"
	"encoding/json"
	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"opensearch-k8-operator/opensearch-gateway/responses"
)

type ClusterDataService struct {
	client   *opensearch.Client
	MainPage responses.MainResponse
}

func NewClusterDataService(config opensearch.Config) (*ClusterDataService, error) {
	service := new(ClusterDataService)
	client, err := opensearch.NewClient(config)
	if err == nil {
		service.client = client
	}
	pingReq := opensearchapi.PingRequest{}
	pingRes, err := pingReq.Do(context.Background(), client)
	if err == nil && pingRes.StatusCode == 200 {
		mainPageResponse, err := mainPage(client)
		if err == nil {
			service.MainPage = mainPageResponse
		}
	}
	return service, err
}

func (service *ClusterDataService) HasIndicesWithNoReplica() (bool, error) {
	req := opensearchapi.CatIndicesRequest{Format: "json"}
	indicesRes, err := req.Do(context.Background(), service.client)
	if err != nil {
		return false, err
	}
	var response []responses.CatIndicesResponse
	defer indicesRes.Body.Close()
	err = json.NewDecoder(indicesRes.Body).Decode(&response)
	if err != nil {
		return false, err
	}
	for _, index := range response {
		if index.Rep == "" || index.Rep == "0" {
			return true, err
		}
	}
	return false, err
}

func mainPage(client *opensearch.Client) (responses.MainResponse, error) {
	req := opensearchapi.InfoRequest{}
	infoRes, err := req.Do(context.Background(), client)
	var response responses.MainResponse
	if err == nil {
		defer infoRes.Body.Close()
		err = json.NewDecoder(infoRes.Body).Decode(&response)
	}
	return response, err
}
