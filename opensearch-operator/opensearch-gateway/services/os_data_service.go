package services

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/go-logr/logr"
	"github.com/opensearch-project/opensearch-go/opensearchutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var ClusterSettingsExcludeBrokenPath = []string{"cluster", "routing", "allocation", "exclude", "_name"}

type ClusterSettingsAllocation string

const (
	ClusterSettingsAllocationPrimaries ClusterSettingsAllocation = "primaries"
	ClusterSettingsAllocationAll       ClusterSettingsAllocation = "all"
	ClusterSettingsAllocationNone      ClusterSettingsAllocation = "none"
)

func HasIndicesWithNoReplica(service *OsClusterClient) (bool, error) {
	response, err := service.CatIndices()
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

func HasShardsOnNode(service *OsClusterClient, nodeName string) (bool, error) {
	var headers []string
	response, err := service.CatShards(headers)
	if err != nil {
		return false, err
	}
	for _, shardsData := range response {
		if shardsData.NodeName == nodeName {
			return true, err
		}
	}
	return false, err
}

func HasIndexPrimariesOnNode(service *OsClusterClient, nodeName string, indices []string) (bool, error) {
	var headers []string
	response, err := service.CatNamedIndicesShards(headers, indices)
	if err != nil {
		return false, err
	}
	for _, shardsData := range response {
		// If primary shards are still initializing consider the node not empty
		if shardsData.PrimaryOrReplica == "p" && shardsData.State != "STARTED" {
			return true, nil
		}
		// If there are system shards on the node consider it not empty
		if shardsData.NodeName == nodeName && shardsData.PrimaryOrReplica == "p" {
			return true, nil
		}
	}
	return false, err
}

func AppendExcludeNodeHost(service *OsClusterClient, nodeNameToExclude string) (bool, error) {
	response, err := service.GetClusterSettings()
	if err != nil {
		return false, err
	}
	val, ok := helpers.FindByPath(response.Transient, ClusterSettingsExcludeBrokenPath)
	valAsString := nodeNameToExclude
	if ok && val != "" {
		// Test whether name is already excluded
		var found bool
		valArr := strings.Split(val.(string), ",")
		for _, name := range valArr {
			if name == nodeNameToExclude {
				found = true
				break
			}
		}
		if !found {
			valArr = append(valArr, nodeNameToExclude)
		}

		valAsString = strings.Join(valArr, ",")
	}
	settings := createClusterSettingsResponseWithExcludeName(valAsString)
	if err == nil {
		_, err = service.PutClusterSettings(settings)
	}
	return err == nil, err
}

func RemoveExcludeNodeHost(service *OsClusterClient, nodeNameToExclude string) (bool, error) {
	response, err := service.GetClusterSettings()
	if err != nil {
		return false, err
	}
	val, ok := helpers.FindByPath(response.Transient, ClusterSettingsExcludeBrokenPath)
	if !ok || val == "" {
		return true, err
	}
	valAsString := strings.ReplaceAll(val.(string), nodeNameToExclude, "")
	valAsString = strings.ReplaceAll(valAsString, ",,", ",")
	settings := createClusterSettingsResponseWithExcludeName(valAsString)
	if err == nil {
		_, err = service.PutClusterSettings(settings)
	}
	return err == nil, err
}

func SetClusterShardAllocation(service *OsClusterClient, enableType ClusterSettingsAllocation) error {
	settings := createClusterSettingsAllocationEnable(enableType)
	_, err := service.PutClusterSettings(settings)
	return err
}

func createClusterSettingsResponseWithExcludeName(exclude string) responses.ClusterSettingsResponse {
	var val *string = nil
	if exclude != "" {
		val = &exclude
	}
	return responses.ClusterSettingsResponse{Transient: map[string]interface{}{
		"cluster": map[string]interface{}{
			"routing": map[string]interface{}{
				"allocation": map[string]interface{}{
					"exclude": map[string]interface{}{
						"_name": val,
					},
				},
			},
		},
	}}
}

func createClusterSettingsAllocationEnable(enable ClusterSettingsAllocation) responses.ClusterSettingsResponse {
	return responses.ClusterSettingsResponse{Transient: map[string]interface{}{
		"cluster": map[string]interface{}{
			"routing": map[string]interface{}{
				"allocation": map[string]interface{}{
					"enable": enable,
				},
			},
		},
	}}
}

func CheckClusterStatusForRestart(service *OsClusterClient, drainNodes bool) (bool, string, error) {
	health, err := service.GetHealth()
	if err != nil {
		return false, "failed to fetch health", err
	}

	if health.Status == "green" {
		return true, "", nil
	}

	if continueRestartWithYellowHealth(health) {
		return true, "", nil
	}

	if drainNodes {
		return false, "cluster is not green and drain nodes is enabled", nil
	}

	flatSettings, err := service.GetFlatClusterSettings()
	if err != nil {
		return false, "could not fetch cluster settings", err
	}

	if flatSettings.Transient.ClusterRoutingAllocationEnable == string(ClusterSettingsAllocationAll) {
		return false, "waiting for health to be green", nil
	}

	// Set shard routing to all
	if err := SetClusterShardAllocation(service, ClusterSettingsAllocationAll); err != nil {
		return false, "failed to set shard allocation", err
	}

	return false, "enabled shard allocation", nil
}

func ReactivateShardAllocation(service *OsClusterClient) error {
	flatSettings, err := service.GetFlatClusterSettings()
	if err != nil {
		return err
	}
	if flatSettings.Transient.ClusterRoutingAllocationEnable == string(ClusterSettingsAllocationAll) {
		return nil
	}

	if err := SetClusterShardAllocation(service, ClusterSettingsAllocationAll); err != nil {
		return err
	}
	return nil
}

func PreparePodForDelete(service *OsClusterClient, lg logr.Logger, podName string, drainNode bool, nodeCount int32) (bool, error) {
	if drainNode {
		// If we are draining nodes then drain the working node
		_, err := AppendExcludeNodeHost(service, podName)
		if err != nil {
			return false, err
		}

		// If there are only 2 data nodes only check for system indices
		if nodeCount == 2 {
			systemIndices, err := GetExistingSystemIndices(service)
			if err != nil {
				return false, err
			}

			systemPrimaries, err := HasIndexPrimariesOnNode(service, podName, systemIndices)
			if err != nil {
				return false, err
			}
			lg.Info(fmt.Sprintf("Waiting to drain primary replicas for system indices from node %s before deleting", podName))
			return !systemPrimaries, nil
		}

		// Check if there are any shards on the node
		nodeNotEmpty, err := HasShardsOnNode(service, podName)
		if err != nil {
			return false, err
		}
		// If the node isn't empty requeue to wait for shards to drain
		lg.Info(fmt.Sprintf("Waiting for node %s to drain before deleting", podName))
		return !nodeNotEmpty, nil
	}
	// Update cluster routing before deleting appropriate ordinal pod
	if err := SetClusterShardAllocation(service, ClusterSettingsAllocationPrimaries); err != nil {
		return false, err
	}
	return true, nil
}

func GetExistingSystemIndices(service *OsClusterClient) ([]string, error) {
	var existing []string
	systemIndices := []string{
		".kibana_1",
		".opendistro_security",
	}
	systemIndices = append(systemIndices, AdditionalSystemIndices...)

	for _, systemIndex := range systemIndices {
		exists, err := service.IndexExists(systemIndex)
		if err != nil {
			return existing, err
		}
		if exists {
			existing = append(existing, systemIndex)
		}
	}

	return existing, nil
}

// continueRestartWithYellowHealth allows upgrades and rolling restarts to continue when the cluster is yellow
// if the yellow status is caused by the .opensearch-observability index.  This is a new index that is created
// on upgrade and will be yellow until at least 2 data nodes are upgraded.
func continueRestartWithYellowHealth(health responses.ClusterHealthResponse) bool {
	if health.Status != "yellow" {
		return false
	}

	if health.RelocatingShards > 0 || health.InitializingShards > 0 || health.UnassignedShards > 1 {
		return false
	}

	observabilityIndex, ok := health.Indices[".opensearch-observability"]
	if !ok {
		return false
	}

	return observabilityIndex.Status == "yellow"
}

// IndexTemplatePath returns a strings.Builder pointing to /_index_template/<templateName>
func IndexTemplatePath(templateName string) strings.Builder {
	var path strings.Builder
	path.Grow(len("/_index_template/") + len(templateName))
	path.WriteString("/_index_template/")
	path.WriteString(templateName)
	return path
}

// IndexTemplateExists checks if the passed index template already exists or not
func IndexTemplateExists(ctx context.Context, service *OsClusterClient, templateName string) (bool, error) {
	path := IndexTemplatePath(templateName)
	resp, err := doHTTPHead(ctx, service.client, path)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return false, nil
	} else if resp.IsError() {
		return false, fmt.Errorf("response from API is %s", resp.Status())
	}
	return true, nil
}

// ShouldUpdateIndexTemplate checks whether a previously created index template needs an update or not
func ShouldUpdateIndexTemplate(
	ctx context.Context,
	service *OsClusterClient,
	indexTemplateName string,
	indexTemplate requests.IndexTemplate,
) (bool, error) {
	path := IndexTemplatePath(indexTemplateName)
	resp, err := doHTTPGet(ctx, service.client, path)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return true, nil
	} else if resp.IsError() {
		return false, fmt.Errorf("response from API is %s", resp.Status())
	}

	indexTemplatesResponse := responses.GetIndexTemplatesResponse{}

	err = json.NewDecoder(resp.Body).Decode(&indexTemplatesResponse)
	if err != nil {
		return false, err
	}

	// we should not be able to get more than one template in the list, but check to make sure
	if len(indexTemplatesResponse.IndexTemplates) != 1 {
		return false, fmt.Errorf("found %d index templates which fits the name '%s'", len(indexTemplatesResponse.IndexTemplates), indexTemplateName)
	}

	indexTemplateResponse := indexTemplatesResponse.IndexTemplates[0]

	// verify the index template name
	if indexTemplateResponse.Name != indexTemplateName {
		return false, fmt.Errorf("returned index template named '%s' does not equal the requested name '%s'", indexTemplateResponse.Name, indexTemplateName)
	}
	if reflect.DeepEqual(indexTemplate, indexTemplateResponse.IndexTemplate) {
		return false, nil
	}

	lg := log.FromContext(ctx)
	lg.Info("OpenSearch Index template requires update")

	return true, nil
}

// CreateOrUpdateIndexTemplate creates a new index or updates a pre-existing index template
func CreateOrUpdateIndexTemplate(
	ctx context.Context,
	service *OsClusterClient,
	indexTemplateName string,
	indexTemplate requests.IndexTemplate,
) error {
	path := IndexTemplatePath(indexTemplateName)

	resp, err := doHTTPPut(ctx, service.client, path, opensearchutil.NewJSONReader(indexTemplate))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return fmt.Errorf("failed to create index template: %s", resp.String())
	}
	return nil
}

// DeleteIndexTemplate deletes a previously created index template
func DeleteIndexTemplate(ctx context.Context, service *OsClusterClient, indexTemplateName string) error {
	path := IndexTemplatePath(indexTemplateName)
	resp, err := doHTTPDelete(ctx, service.client, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return fmt.Errorf("response from API is %s", resp.Status())
	}
	return nil
}

// ComponentTemplatePath returns a strings.Builder pointing to /_component_template/<templateName>
func ComponentTemplatePath(templateName string) strings.Builder {
	var path strings.Builder
	path.Grow(len("/_component_template/") + len(templateName))
	path.WriteString("/_component_template/")
	path.WriteString(templateName)
	return path
}

// ComponentTemplateExists checks if the passed component template already exists or not
func ComponentTemplateExists(ctx context.Context, service *OsClusterClient, templateName string) (bool, error) {
	path := ComponentTemplatePath(templateName)
	resp, err := doHTTPHead(ctx, service.client, path)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return false, nil
	} else if resp.IsError() {
		return false, fmt.Errorf("response from API is %s", resp.Status())
	}
	return true, nil
}

// ShouldUpdateComponentTemplate checks whether a previously created component template needs an update or not
func ShouldUpdateComponentTemplate(
	ctx context.Context,
	service *OsClusterClient,
	componentTemplateName string,
	componentTemplate requests.ComponentTemplate,
) (bool, error) {
	path := ComponentTemplatePath(componentTemplateName)
	resp, err := doHTTPGet(ctx, service.client, path)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return true, nil
	} else if resp.IsError() {
		return false, fmt.Errorf("response from API is %s", resp.Status())
	}

	componentTemplatesResponse := responses.GetComponentTemplatesResponse{}

	err = json.NewDecoder(resp.Body).Decode(&componentTemplatesResponse)
	if err != nil {
		return false, err
	}

	// we should not be able to get more than one template in the list, but check to make sure
	if len(componentTemplatesResponse.ComponentTemplates) != 1 {
		return false, fmt.Errorf("found %d component templates which fits the name '%s'", len(componentTemplatesResponse.ComponentTemplates), componentTemplateName)
	}

	componentTemplateResponse := componentTemplatesResponse.ComponentTemplates[0]

	// verify the component template name
	if componentTemplateResponse.Name != componentTemplateName {
		return false, fmt.Errorf("returned component template named '%s' does not equal the requested name '%s'", componentTemplateResponse.Name, componentTemplateName)
	}

	if reflect.DeepEqual(componentTemplate, componentTemplateResponse.ComponentTemplate) {
		return false, nil
	}

	lg := log.FromContext(ctx)
	lg.Info("OpenSearch Component template requires update")

	return true, nil
}

// CreateOrUpdateComponentTemplate creates a new component or updates a pre-existing component template
func CreateOrUpdateComponentTemplate(
	ctx context.Context,
	service *OsClusterClient,
	componentTemplateName string,
	componentTemplate requests.ComponentTemplate,
) error {
	path := ComponentTemplatePath(componentTemplateName)

	resp, err := doHTTPPut(ctx, service.client, path, opensearchutil.NewJSONReader(componentTemplate))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return fmt.Errorf("failed to create component template: %s", resp.String())
	}
	return nil
}

// DeleteComponentTemplate deletes a previously created component template
func DeleteComponentTemplate(ctx context.Context, service *OsClusterClient, componentTemplateName string) error {
	path := ComponentTemplatePath(componentTemplateName)
	resp, err := doHTTPDelete(ctx, service.client, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return fmt.Errorf("response from API is %s", resp.Status())
	}
	return nil
}
