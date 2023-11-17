package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"github.com/opensearch-project/opensearch-go/opensearchutil"
	"k8s.io/utils/pointer"
)

const (
	headerContentType = "Content-Type"

	jsonContentHeader = "application/json"
	ismResource       = "_ism"
)

var AdditionalSystemIndices = []string{
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

type OsClusterClient struct {
	OsClusterClientOptions
	client   *opensearch.Client
	MainPage responses.MainResponse
}

type OsClusterClientOptions struct {
	transport http.RoundTripper
}

type OsClusterClientOption func(*OsClusterClientOptions)

func (o *OsClusterClientOptions) apply(opts ...OsClusterClientOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithTransport(transport http.RoundTripper) OsClusterClientOption {
	return func(o *OsClusterClientOptions) {
		o.transport = transport
	}
}

func NewOsClusterClient(clusterUrl string, username string, password string, opts ...OsClusterClientOption) (*OsClusterClient, error) {
	options := OsClusterClientOptions{}
	options.apply(opts...)
	config := opensearch.Config{
		Transport: func() http.RoundTripper {
			if options.transport != nil {
				return options.transport
			}
			return &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}(),
		Addresses: []string{clusterUrl},
		Username:  username,
		Password:  password,
	}

	client, err := NewOsClusterClientFromConfig(config)
	if err != nil {
		return nil, err
	}

	client.OsClusterClientOptions = options
	return client, nil
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

func (client *OsClusterClient) GetHealth() (responses.ClusterHealthResponse, error) {
	req := opensearchapi.ClusterHealthRequest{
		Level: "indices",
	}
	catNodesRes, err := req.Do(context.Background(), client.client)
	var response responses.ClusterHealthResponse
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
		FlatSettings: pointer.Bool(true),
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

// GetSecurityResource performs an HTTP GET request to OS to fetch the security resource specified by name
func (client *OsClusterClient) GetSecurityResource(ctx context.Context, resource, name string) (*opensearchapi.Response, error) {
	path := generateAPIPath(resource, name)
	return doHTTPGet(ctx, client.client, path)
}

// PutSecurityResource performs an HTTP PUT request to OS to create/update the security resource specified by name
func (client *OsClusterClient) PutSecurityResource(ctx context.Context, resource, name string, body io.Reader) (*opensearchapi.Response, error) {
	path := generateAPIPath(resource, name)
	return doHTTPPut(ctx, client.client, path, body)
}

// DeleteSecurityResource performs an HTTP DELETE request to OS to delete the security resource specified by name
func (client *OsClusterClient) DeleteSecurityResource(ctx context.Context, resource, name string) (*opensearchapi.Response, error) {
	path := generateAPIPath(resource, name)
	return doHTTPDelete(ctx, client.client, path)
}

// GetISMConfig performs an HTTP GET request to OS to get the ISM policy resource specified by name
func (client *OsClusterClient) GetISMConfig(ctx context.Context, name string) (*opensearchapi.Response, error) {
	path := generateAPIPathISM(ismResource, name)
	return doHTTPGet(ctx, client.client, path)
}

// PutISMConfig performs an HTTP PUT request to OS to create the ISM policy resource specified by name
func (client *OsClusterClient) PutISMConfig(ctx context.Context, name string, body io.Reader) (*opensearchapi.Response, error) {
	path := generateAPIPathISM(ismResource, name)
	return doHTTPPut(ctx, client.client, path, body)
}

// UpdateISMConfig performs an HTTP PUT request to OS to update the ISM policy resource specified by name
func (client *OsClusterClient) UpdateISMConfig(ctx context.Context, name string, seqnumber, primterm int, body io.Reader) (*opensearchapi.Response, error) {
	path := generateAPIPathUpdateISM(ismResource, name, seqnumber, primterm)
	return doHTTPPut(ctx, client.client, path, body)
}

// DeleteISMConfig performs an HTTP DELETE request to OS to delete the ISM policy resource specified by name
func (client *OsClusterClient) DeleteISMConfig(ctx context.Context, name string) (*opensearchapi.Response, error) {
	path := generateAPIPathISM(ismResource, name)
	return doHTTPDelete(ctx, client.client, path)
}

// generateAPIPathISM generates a URI PATH for a specific resource endpoint and name
// For example: resource = _ism, name = example
// URI PATH = '_plugins/_ism/policies/example'
func generateAPIPathISM(resource, name string) strings.Builder {
	var path strings.Builder
	path.Grow(1 + len("_plugins") + 1 + len(resource) + 1 + len("policies") + 1 + len(name))
	path.WriteString("/")
	path.WriteString("_plugins")
	path.WriteString("/")
	path.WriteString(resource)
	path.WriteString("/")
	path.WriteString("policies")
	path.WriteString("/")
	path.WriteString(name)
	return path
}

// generateAPIPathUpdateISM generates a URI PATH for ISM policy resource endpoint and name
// For example: resource = _ism, name = example, seq_no = 7, primary_term = 1
// URI PATH = '_plugins/_ism/policies/example?if_seq_no=7&if_primary_term=1'
func generateAPIPathUpdateISM(resource, name string, seqno, primaryterm int) strings.Builder {
	var path strings.Builder
	path.Grow(1 + len("_plugins") + 1 + len(resource) + 1 + len("policies") + 1 + len(name) + len("?if_seq_no=") + len(strconv.Itoa(seqno)) + len("&if_primary_term=") + len(strconv.Itoa(primaryterm)))
	path.WriteString("/")
	path.WriteString("_plugins")
	path.WriteString("/")
	path.WriteString(resource)
	path.WriteString("/")
	path.WriteString("policies")
	path.WriteString("/")
	path.WriteString(name)
	path.WriteString("?if_seq_no=")
	path.WriteString(strconv.Itoa(seqno))
	path.WriteString("&if_primary_term=")
	path.WriteString(strconv.Itoa(primaryterm))
	return path
}

// generateAPIPath generates a URI PATH for a specific resource endpoint and name
// For example: resource = internalusers, name = example
// URI PATH = '_plugins/_security/api/internalusers/example'
func generateAPIPath(resource, name string) strings.Builder {
	var path strings.Builder
	path.Grow(1 + len("_plugins") + 1 + len("_security") + 1 + len("api") + 1 + len(resource) + 1 + len(name))
	path.WriteString("/")
	path.WriteString("_plugins")
	path.WriteString("/")
	path.WriteString("_security")
	path.WriteString("/")
	path.WriteString("api")
	path.WriteString("/")
	path.WriteString(resource)
	path.WriteString("/")
	path.WriteString(name)
	return path
}
