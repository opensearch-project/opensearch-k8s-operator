package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

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
	_, err = service.PutClusterSettings(settings)
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
	valAsString := val.(string)
	// Split the comma-separated list, filter out the node to exclude, and rejoin
	valArr := strings.Split(valAsString, ",")
	var filteredArr []string
	for _, name := range valArr {
		trimmedName := strings.TrimSpace(name)
		if trimmedName != "" && trimmedName != nodeNameToExclude {
			filteredArr = append(filteredArr, trimmedName)
		}
	}
	valAsString = strings.Join(filteredArr, ",")
	settings := createClusterSettingsResponseWithExcludeName(valAsString)
	_, err = service.PutClusterSettings(settings)
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
	defer helpers.SafeClose(resp.Body)

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
	defer helpers.SafeClose(resp.Body)

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

	if indexTemplateResponse.IndexTemplate.Template.Settings != nil {
		indexTemplateResponse.IndexTemplate.Template.Settings, err = helpers.SortedJsonKeys(indexTemplateResponse.IndexTemplate.Template.Settings)
		if err != nil {
			return false, err
		}
	}

	if indexTemplateResponse.IndexTemplate.Template.Mappings != nil {
		indexTemplateResponse.IndexTemplate.Template.Mappings, err = helpers.SortedJsonKeys(indexTemplateResponse.IndexTemplate.Template.Mappings)
		if err != nil {
			return false, err
		}
	}

	if cmp.Equal(indexTemplate, indexTemplateResponse.IndexTemplate, cmpopts.EquateEmpty()) {
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
	defer helpers.SafeClose(resp.Body)

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
	defer helpers.SafeClose(resp.Body)

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
	defer helpers.SafeClose(resp.Body)

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
	defer helpers.SafeClose(resp.Body)

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

	if componentTemplateResponse.ComponentTemplate.Template.Settings != nil {
		componentTemplateResponse.ComponentTemplate.Template.Settings, err = helpers.SortedJsonKeys(componentTemplateResponse.ComponentTemplate.Template.Settings)
		if err != nil {
			return false, err
		}
	}

	if componentTemplateResponse.ComponentTemplate.Template.Mappings != nil {
		componentTemplateResponse.ComponentTemplate.Template.Mappings, err = helpers.SortedJsonKeys(componentTemplateResponse.ComponentTemplate.Template.Mappings)
		if err != nil {
			return false, err
		}
	}

	if cmp.Equal(componentTemplate, componentTemplateResponse.ComponentTemplate, cmpopts.EquateEmpty()) {
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
	defer helpers.SafeClose(resp.Body)

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
	defer helpers.SafeClose(resp.Body)

	if resp.IsError() {
		return fmt.Errorf("response from API is %s", resp.Status())
	}
	return nil
}

// Determines which of the current settings will not be supported by the
// new version.
// This function maintains a list of deprecated/removed cluster settings per major version.
// When upgrading between major versions (e.g., 2.x -> 3.x, 3.x -> 4.x), these settings
// must be removed before the upgrade to prevent deadlocks and errors.
// The list is maintained per major version upgrade and will be extended for future upgrades.
func DetermineUnsupportedClusterSettings(newVersion string) (responses.ClusterSettingsResponse, error) {
	settingsToDelete := responses.ClusterSettingsResponse{
		Transient:  make(map[string]interface{}),
		Persistent: make(map[string]interface{}),
	}
	// Settings removed per major version upgrade
	// This list will be extended for future major version upgrades (e.g., 4.0.0, 5.0.0)
	var removedSettingsByVersion = map[string][]string{
		"3.0.0": {
			// Settings removed when upgrading from 2.x to 3.x
			// https://github.com/opensearch-project/index-management/pull/963
			"archived.opendistro.index_state_management.metadata_service.enabled",
			"archived.opendistro.index_state_management.metadata_migration.status",
			"archived.opendistro.index_state_management.template_migration.control",
			"archived.plugins.index_state_management.metadata_service.enabled",
			"archived.plugins.index_state_management.metadata_migration.status",
			"archived.plugins.index_state_management.template_migration.control",
		},
	}

	// Parse version
	new, err := semver.NewVersion(newVersion)
	if err != nil {
		return settingsToDelete, err
	}

	// Determine settings to delete
	for minVer, removedSettings := range removedSettingsByVersion {
		newerThanMin, err := semver.NewConstraint(fmt.Sprintf(">= %s", minVer))
		if err != nil {
			return settingsToDelete, err
		}

		// Check if the new version is newer than the group we're checking
		if newerThanMin.Check(new) {
			// Schedule the settings for removal
			for _, settingName := range removedSettings {
				settingsToDelete.Transient[settingName] = nil
				settingsToDelete.Persistent[settingName] = nil
			}
		}
	}

	return settingsToDelete, nil
}

// Before performing an upgrade, deprecated settings that will be removed
// in the upgraded version shall be removed in order not to cause a deadlock.
func DeleteUnsupportedClusterSettings(service *OsClusterClient, newVersion string) error {
	settingsToDelete, err := DetermineUnsupportedClusterSettings(newVersion)
	if err != nil {
		return err
	}

	_, err = service.PutClusterSettings(settingsToDelete)
	return err
}
