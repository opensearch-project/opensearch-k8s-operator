package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"io"
	"k8s.io/utils/ptr"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"github.com/opensearch-project/opensearch-go/opensearchutil"
)

const (
	headerContentType = "Content-Type"

	jsonContentHeader      = "application/json"
	ismResource            = "_ism"
	snapshotpolicyResource = "_sm"
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
				// These options are needed as otherwise connections would be kept and leak memory
				// Connection reuse is not really possible due to each reconcile run being independent
				DisableKeepAlives: true,
				MaxIdleConns:      1,
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
	} else {
		return nil, err
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
		defer helpers.SafeClose(infoRes.Body)
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
		defer helpers.SafeClose(catNodesRes.Body)
		err = json.NewDecoder(catNodesRes.Body).Decode(&response)
	}
	return response, err
}

func (client *OsClusterClient) CatNodes() ([]responses.CatNodesResponse, error) {
	req := opensearchapi.CatNodesRequest{Format: "json"}
	catNodesRes, err := req.Do(context.Background(), client.client)
	var response []responses.CatNodesResponse
	if err == nil {
		defer helpers.SafeClose(catNodesRes.Body)
		err = json.NewDecoder(catNodesRes.Body).Decode(&response)
	}
	return response, err
}

func (client *OsClusterClient) NodesStats() (responses.NodesStatsResponse, error) {
	req := opensearchapi.NodesStatsRequest{}
	catNodesRes, err := req.Do(context.Background(), client.client)
	var response responses.NodesStatsResponse
	if err == nil {
		defer helpers.SafeClose(catNodesRes.Body)
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
	defer helpers.SafeClose(indicesRes.Body)
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
	defer helpers.SafeClose(indicesRes.Body)
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
	defer helpers.SafeClose(indicesRes.Body)
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
	defer helpers.SafeClose(settingsRes.Body)
	err = json.NewDecoder(settingsRes.Body).Decode(&response)
	return response, err
}

func (client *OsClusterClient) GetFlatClusterSettings() (responses.FlatClusterSettingsResponse, error) {
	req := opensearchapi.ClusterGetSettingsRequest{
		FlatSettings: ptr.To(true),
	}
	settingsRes, err := req.Do(context.Background(), client.client)
	var response responses.FlatClusterSettingsResponse
	if err != nil {
		return response, err
	}
	defer helpers.SafeClose(settingsRes.Body)

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
	defer helpers.SafeClose(settingsRes.Body)
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
	defer helpers.SafeClose(settingsRes.Body)
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
	defer helpers.SafeClose(resp.Body)

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
	defer helpers.SafeClose(indicesRes.Body)
	if indicesRes.StatusCode == 404 {
		return false, nil
	} else if indicesRes.IsError() {
		return false, ErrCatIndicesFailed(indicesRes.String())
	}

	return true, nil
}

// GetIndices retrieves indices matching the given pattern from OpenSearch
func (client *OsClusterClient) GetIndices(ctx context.Context, pattern string) (*opensearchapi.Response, error) {
	path := generateGetIndicesPath(pattern)
	return doHTTPGet(ctx, client.client, path)
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

// AddPolicyToIndex performs an HTTP POST request to OS to add an ISM policy to an index
func (client *OsClusterClient) AddPolicyToIndex(ctx context.Context, indexName string, body io.Reader) (*opensearchapi.Response, error) {
	path := generateAPIPathAddISMPolicyToIndex(ismResource, indexName)
	return doHTTPPost(ctx, client.client, path, body)
}

// performs an HTTP GET request to OS to get the snapshot repository specified by name
func (client *OsClusterClient) GetSnapshotRepository(ctx context.Context, name string) (*opensearchapi.Response, error) {
	path := generateAPIPathSnapshotRepository(name)
	return doHTTPGet(ctx, client.client, path)
}

// performs an HTTP PUT request to OS to create the snapshot repository specified by name
func (client *OsClusterClient) CreateSnapshotRepository(ctx context.Context, name string, body io.Reader) (*opensearchapi.Response, error) {
	path := generateAPIPathSnapshotRepository(name)
	return doHTTPPut(ctx, client.client, path, body)
}

// performs an HTTP PUT request to OS to update the snapshot repository specified by name
func (client *OsClusterClient) UpdateSnapshotRepository(ctx context.Context, name string, body io.Reader) (*opensearchapi.Response, error) {
	path := generateAPIPathSnapshotRepository(name)
	return doHTTPPut(ctx, client.client, path, body)
}

// DeleteISMConfig performs an HTTP DELETE request to OS to delete the ISM policy resource specified by name
func (client *OsClusterClient) DeleteSnapshotRepository(ctx context.Context, name string) (*opensearchapi.Response, error) {
	path := generateAPIPathSnapshotRepository(name)
	return doHTTPDelete(ctx, client.client, path)
}

// GetSnapshotPolicyConfig performs an HTTP GET request to OS to create the Snapshot policy resource specified by name
func (client *OsClusterClient) GetSnapshotPolicyConfig(ctx context.Context, name string) (*opensearchapi.Response, error) {
	path := generateAPIPathSnapshotPolicies(snapshotpolicyResource, name)
	return doHTTPGet(ctx, client.client, path)
}

// CreateSnapshotPolicyConfig performs an HTTP POST request to OS to create the Snapshot policy resource specified by name
func (client *OsClusterClient) CreateSnapshotPolicyConfig(ctx context.Context, name string, body io.Reader) (*opensearchapi.Response, error) {
	path := generateAPIPathSnapshotPolicies(snapshotpolicyResource, name)
	return doHTTPPost(ctx, client.client, path, body)
}

// DeleteSnapshotPolicyConfig performs an HTTP DELETE request to OS to delete the Snapshot policy resource specified by name
func (client *OsClusterClient) DeleteSnapshotPolicyConfig(ctx context.Context, name string) (*opensearchapi.Response, error) {
	path := generateAPIPathSnapshotPolicies(snapshotpolicyResource, name)
	return doHTTPDelete(ctx, client.client, path)
}

// UpdateSnapshotPolicyConfig performs an HTTP PUT request to OS to update the Snapshot policy resource specified by name
func (client *OsClusterClient) UpdateSnapshotPolicyConfig(ctx context.Context, name string, seqnumber, primterm int, body io.Reader) (*opensearchapi.Response, error) {
	path := generateAPIPathSnapshotUpdatePolicies(snapshotpolicyResource, name, seqnumber, primterm)
	return doHTTPPut(ctx, client.client, path, body)
}

// generateGetIndicesPath generates a URI PATH for a specific resource endpoint and name
// For example: pattern = example-*
// URI PATH = '_cat/indices/example-*?format=json'
func generateGetIndicesPath(pattern string) strings.Builder {
	var path strings.Builder
	path.Grow(1 + len("_cat") + 1 + len("indices") + 1 + len(pattern) + len("?format=json"))
	path.WriteString("/")
	path.WriteString("_cat")
	path.WriteString("/")
	path.WriteString("indices")
	path.WriteString("/")
	path.WriteString(pattern)
	path.WriteString("?format=json")
	return path
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

// generateAPIPathAddISMPolicyToIndex generates a URI PATH for adding ISM policy to an index
// URI PATH = '_plugins/_ism/add/<indexName>'
func generateAPIPathAddISMPolicyToIndex(resource, indexName string) strings.Builder {
	var path strings.Builder
	path.Grow(1 + len("_plugins") + 1 + len(resource) + 1 + len("add") + 1 + len(indexName))
	path.WriteString("/")
	path.WriteString("_plugins")
	path.WriteString("/")
	path.WriteString(resource)
	path.WriteString("/")
	path.WriteString("add")
	path.WriteString("/")
	path.WriteString(indexName)
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

// generates a URI PATH for a given snapshot repository name
func generateAPIPathSnapshotRepository(name string) strings.Builder {
	var path strings.Builder
	path.Grow(1 + len("_snapshot") + 1 + len(name))
	path.WriteString("/")
	path.WriteString("_snapshot")
	path.WriteString("/")
	path.WriteString(name)
	return path
}

// generateAPIPathSnapshotPolicies generates a URI PATH for a specific resource endpoint and name
// For example: resource = _sm, name = example
// URI PATH = '_plugins/_sm/policies/example'
func generateAPIPathSnapshotPolicies(resource, name string) strings.Builder {
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

// generateAPIPathSnapshotPolicies generates a URI PATH for a specific resource endpoint and name for updating the resource
// For example: resource = _sm, name = example, seqno = 1, primaryterm = 0
// URI PATH = '_plugins/_sm/policies/example'
func generateAPIPathSnapshotUpdatePolicies(resource, name string, seqno, primaryterm int) strings.Builder {
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
