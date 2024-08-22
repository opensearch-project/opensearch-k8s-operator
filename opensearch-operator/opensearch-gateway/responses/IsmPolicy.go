package responses

import "github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"

type GetISMPolicyResponse struct {
	PolicyID       string `json:"_id"`
	PrimaryTerm    int    `json:"_primary_term"`
	SequenceNumber int    `json:"_seq_no"`
	Policy         requests.ISMPolicySpec
}
