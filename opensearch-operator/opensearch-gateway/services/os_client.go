package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"net/http"
	"opensearch.opster.io/opensearch-gateway/responses"
	"strings"
)

type OsClusterClient struct {
	client   *opensearch.Client
	MainPage responses.MainResponse
}

func NewOsClusterClient(clusterUrl string, username string, password string) (*OsClusterClient, error) {
	config := opensearch.Config{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Addresses: []string{clusterUrl},
		Username:  username,
		Password:  password,
	}
	return NewOsClusterClientFromConfig(config)
}

func NewOsClusterClientFromConfig(config opensearch.Config) (*OsClusterClient, error) {
	service := new(OsClusterClient)
	client, err := opensearch.NewClient(config)
	if err == nil {
		service.client = client
	}
	pingReq := opensearchapi.PingRequest{}
	pingRes, err := pingReq.Do(context.Background(), client)
	if err == nil && pingRes.StatusCode == 200 {
		mainPageResponse, err := MainPage(client)
		if err == nil {
			service.MainPage = mainPageResponse
		}
	}
	return service, err
}

func MainPage(client *opensearch.Client) (responses.MainResponse, error) {
	req := opensearchapi.InfoRequest{}
	infoRes, err := req.Do(context.Background(), client)
	var response responses.MainResponse
	if err == nil {
		defer infoRes.Body.Close()
		err = json.NewDecoder(infoRes.Body).Decode(&response)
	}
	return response, err
}

func (client *OsClusterClient) CatHealth() (responses.CatHealthResponse, error) {
	req := opensearchapi.CatHealthRequest{Format: "json"}
	catNodesRes, err := req.Do(context.Background(), client.client)
	var response responses.CatHealthResponse
	if err == nil {
		defer catNodesRes.Body.Close()
		err = json.NewDecoder(catNodesRes.Body).Decode(&response)
	}
	return response, err
}

func (client *OsClusterClient) CatNodes() (responses.CatNodesResponse, error) {
	req := opensearchapi.CatNodesRequest{Format: "json"}
	catNodesRes, err := req.Do(context.Background(), client.client)
	var response responses.CatNodesResponse
	if err == nil {
		defer catNodesRes.Body.Close()
		err = json.NewDecoder(catNodesRes.Body).Decode(&response)
	}
	return response, err
}

func (client *OsClusterClient) NodesStats() (responses.NodesStatsResponse, error) {
	req := opensearchapi.NodesStatsRequest{}
	catNodesRes, err := req.Do(context.Background(), client.client)
	var response responses.NodesStatsResponse
	if err == nil {
		defer catNodesRes.Body.Close()
		err = json.NewDecoder(catNodesRes.Body).Decode(&response)
	}
	return response, err
}

func (client *OsClusterClient) CatIndices() ([]responses.CatIndicesResponse, error) {
	req := opensearchapi.CatIndicesRequest{Format: "json"}
	indicesRes, err := req.Do(context.Background(), client.client)
	var response []responses.CatIndicesResponse
	if err != nil {
		return response, err
	}
	defer indicesRes.Body.Close()
	err = json.NewDecoder(indicesRes.Body).Decode(&response)
	return response, err
}

func (client *OsClusterClient) CatShards(headers []string) ([]responses.CatShardsResponse, error) {
	req := opensearchapi.CatShardsRequest{Format: "json", H: headers}
	indicesRes, err := req.Do(context.Background(), client.client)
	var response []responses.CatShardsResponse
	if err != nil {
		return response, err
	}
	defer indicesRes.Body.Close()
	err = json.NewDecoder(indicesRes.Body).Decode(&response)
	return response, err
}

func (client *OsClusterClient) GetClusterSettings() (responses.ClusterSettingsResponse, error) {
	req := opensearchapi.ClusterGetSettingsRequest{Pretty: true}
	settingsRes, err := req.Do(context.Background(), client.client)
	var response responses.ClusterSettingsResponse
	if err != nil {
		return response, err
	}
	defer settingsRes.Body.Close()
	err = json.NewDecoder(settingsRes.Body).Decode(&response)
	return response, err
}

func (client *OsClusterClient) PutClusterSettings(settingsJson string) (responses.ClusterSettingsResponse, error) {
	body := strings.NewReader(settingsJson)
	req := opensearchapi.ClusterPutSettingsRequest{Body: body}
	settingsRes, err := req.Do(context.Background(), client.client)
	var response responses.ClusterSettingsResponse
	if err != nil {
		return response, err
	}
	defer settingsRes.Body.Close()
	err = json.NewDecoder(settingsRes.Body).Decode(&response)
	return response, err
}

func (client *OsClusterClient) ReRouteShard(rerouteJson string) (responses.ClusterRerouteResponse, error) {
	body := strings.NewReader(rerouteJson)
	req := opensearchapi.ClusterRerouteRequest{Body: body}
	settingsRes, err := req.Do(context.Background(), client.client)
	var response responses.ClusterRerouteResponse
	if err != nil {
		return response, err
	}
	defer settingsRes.Body.Close()
	err = json.NewDecoder(settingsRes.Body).Decode(&response)
	return response, err
}
