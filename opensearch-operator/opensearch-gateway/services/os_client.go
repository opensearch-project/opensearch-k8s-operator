package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"github.com/opensearch-project/opensearch-go/opensearchutil"
	"k8s.io/utils/pointer"
	"opensearch.opster.io/opensearch-gateway/responses"
)

var (
	AdditionalSystemIndices = []string{
		".opendistro-alerting-config",
		".opendistro-alerting-alert*",
		".opendistro-anomaly-results*",
		".opendistro-anomaly-detector*",
		".opendistro-anomaly-checkpoints",
		".opendistro-anomaly-detection-state",
		".opendistro-reports-*",
		".opendistro-notifications-*",
		".opendistro-notebooks",
		".opensearch-observability",
		".opendistro-asynchronous-search-response*",
		".replication-metadata-store",
	}
)

type OsClusterClient struct {
	client        *opensearch.Client
	MainPage      responses.MainResponse
	systemIndices []string
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

func (client *OsClusterClient) GetHealth() (responses.CatHealthResponse, error) {
	req := opensearchapi.ClusterHealthRequest{}
	catNodesRes, err := req.Do(context.Background(), client.client)
	var response responses.CatHealthResponse
	if err == nil {
		defer catNodesRes.Body.Close()
		err = json.NewDecoder(catNodesRes.Body).Decode(&response)
	}
	return response, err
}

func (client *OsClusterClient) CatNodes() ([]responses.CatNodesResponse, error) {
	req := opensearchapi.CatNodesRequest{Format: "json"}
	catNodesRes, err := req.Do(context.Background(), client.client)
	var response []responses.CatNodesResponse
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

func (client *OsClusterClient) CatNamedIndicesShards(headers []string, indices []string) ([]responses.CatShardsResponse, error) {
	req := opensearchapi.CatShardsRequest{
		Index:  indices,
		Format: "json",
		H:      headers,
	}
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

func (client *OsClusterClient) GetFlatClusterSettings() (responses.FlatClusterSettingsResponse, error) {
	req := opensearchapi.ClusterGetSettingsRequest{
		FlatSettings: pointer.BoolPtr(true),
	}
	settingsRes, err := req.Do(context.Background(), client.client)
	var response responses.FlatClusterSettingsResponse
	if err != nil {
		return response, err
	}
	defer settingsRes.Body.Close()

	if settingsRes.IsError() {
		return response, ErrClusterHealthGetFailed(settingsRes.String())
	}

	err = json.NewDecoder(settingsRes.Body).Decode(&response)
	return response, err
}

func (client *OsClusterClient) PutClusterSettings(settings responses.ClusterSettingsResponse) (responses.ClusterSettingsResponse, error) {
	body := opensearchutil.NewJSONReader(settings)
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

func (client *OsClusterClient) GetClusterHealth() (responses.ClusterHealthResponse, error) {
	req := opensearchapi.ClusterHealthRequest{
		Timeout: 10 * time.Second,
	}

	health := responses.ClusterHealthResponse{}
	resp, err := req.Do(context.Background(), client.client)
	if err != nil {
		return health, err
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return health, ErrClusterHealthGetFailed(resp.String())
	}

	err = json.NewDecoder(resp.Body).Decode(&health)
	return health, err
}

func (client *OsClusterClient) IndexExists(indexName string) (bool, error) {
	req := opensearchapi.CatIndicesRequest{
		Format: "json",
		Index: []string{
			indexName,
		},
	}
	indicesRes, err := req.Do(context.Background(), client.client)
	if err != nil {
		return false, err
	}
	defer indicesRes.Body.Close()
	if indicesRes.StatusCode == 404 {
		return false, nil
	} else if indicesRes.IsError() {
		return false, ErrCatIndicesFailed(indicesRes.String())
	}

	return true, nil
}
