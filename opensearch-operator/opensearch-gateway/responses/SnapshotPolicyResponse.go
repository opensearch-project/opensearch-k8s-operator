package responses

import "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"

type GetSnapshotPolicyResponse struct {
	PolicyId       string                      `json:"_id"`
	PrimaryTerm    int                         `json:"_primary_term"`
	SequenceNumber int                         `json:"_seq_no"`
	Version        int                         `json:"_version"`
	Policy         requests.SnapshotPolicySpec `json:"sm_policy"`
}
