package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"

	"github.com/opensearch-project/opensearch-go/opensearchutil"

	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
)

// GetSearchTemplate gets the search template with the given name
func GetSearchTemplate(ctx context.Context, service *OsClusterClient, policyName string) (*responses.GetSearchTemplateResponse, error) {
	resp, err := service.GetSearchTemplateConfig(ctx, policyName)
	if err != nil {
		return nil, err
	}
	defer helpers.SafeClose(resp.Body)
	if resp.StatusCode == 404 {
		return nil, ErrNotFound
	}
	if resp.IsError() {
		return nil, fmt.Errorf("response from API is %s", resp.Status())
	}
	snapshotPolicyResponse := responses.GetSearchTemplateResponse{}
	if resp != nil && resp.Body != nil {
		err := json.NewDecoder(resp.Body).Decode(&snapshotPolicyResponse)
		if err != nil {
			return nil, err
		}
		return &snapshotPolicyResponse, nil
	}
	return nil, fmt.Errorf("response is empty")
}

// CreateSearchTemplate creates the passed search template
func CreateSearchTemplate(ctx context.Context, service *OsClusterClient, searchtemplate requests.SearchTemplateSpec, policyName string) error {
	spec := opensearchutil.NewJSONReader(searchtemplate)
	resp, err := service.CreateSearchTemplateConfig(ctx, policyName, spec)
	if err != nil {
		return err
	}
	defer helpers.SafeClose(resp.Body)
	if resp.IsError() {
		return fmt.Errorf("failed to create search template: %s", resp.String())
	}
	return nil
}

// DeleteSearchTemplate deletes the given search template
func DeleteSearchTemplate(ctx context.Context, service *OsClusterClient, searchTemplate string) error {
	resp, err := service.DeleteSearchTemplateConfig(ctx, searchTemplate)
	if err != nil {
		return err
	}
	defer helpers.SafeClose(resp.Body)
	if resp.IsError() {
		return fmt.Errorf("failed to delete search template: %s", resp.String())
	}
	return nil
}

// UpdateSearchTemplate updates the given policy
func UpdateSearchTemplate(ctx context.Context, service *OsClusterClient, searchtemplate requests.SearchTemplateSpec, policyName string) error {
	spec := opensearchutil.NewJSONReader(searchtemplate)
	resp, err := service.UpdateSearchTemplateConfig(ctx, policyName, spec)
	if err != nil {
		return err
	}
	defer helpers.SafeClose(resp.Body)
	if resp.IsError() {
		return fmt.Errorf("failed to update search template: %s", resp.String())
	}
	return nil
}
