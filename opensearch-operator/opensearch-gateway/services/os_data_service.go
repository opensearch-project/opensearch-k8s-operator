package services

import (
	"strings"

	"opensearch.opster.io/opensearch-gateway/responses"
	"opensearch.opster.io/pkg/helpers"
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
	var valAsString = nodeNameToExclude
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

func PreparePodForDelete(service *OsClusterClient, podName string, drainNode bool, nodeCount int32) (bool, error) {
	if drainNode {
		// If we are draining nodes then drain the working node
		_, err := AppendExcludeNodeHost(service, podName)
		if err != nil {
			return false, err
		}

		// If there are only 2 data nodes only check for system indics
		if nodeCount == 2 {
			systemIndices, err := GetExistingSystemIndices(service)
			if err != nil {
				return false, err
			}

			systemPrimaries, err := HasIndexPrimariesOnNode(service, podName, systemIndices)
			if err != nil {
				return false, err
			}
			return !systemPrimaries, nil
		}

		// Check if there are any shards on the node
		nodeNotEmpty, err := HasShardsOnNode(service, podName)
		if err != nil {
			return false, err
		}
		// If the node isn't empty requeue to wait for shards to drain
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
