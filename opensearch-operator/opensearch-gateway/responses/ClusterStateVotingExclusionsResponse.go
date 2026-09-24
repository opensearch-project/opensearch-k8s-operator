package responses

// ClusterStateVotingExclusionsResponse is the shape of
// GET /_cluster/state/metadata?filter_path=metadata.cluster_coordination.voting_config_exclusions
type ClusterStateVotingExclusionsResponse struct {
	Metadata struct {
		ClusterCoordination struct {
			VotingConfigExclusions []VotingConfigExclusion `json:"voting_config_exclusions"`
		} `json:"cluster_coordination"`
	} `json:"metadata"`
}

// VotingConfigExclusion is one entry of the voting configuration exclusions list.
// NodeId is "_absent_" for an exclusion added by name for a node that was not in
// the cluster at the time.
type VotingConfigExclusion struct {
	NodeId   string `json:"node_id"`
	NodeName string `json:"node_name"`
}
