package responses

import "github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"

type GetSearchTemplateResponse struct {
	PolicyId string                        `json:"_id"`
	Found    bool                          `json:"found"`
	Script   requests.SearchTemplateScript `json:"script"`
}
