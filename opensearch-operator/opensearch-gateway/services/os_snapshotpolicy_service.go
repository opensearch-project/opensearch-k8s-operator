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

// GetSnapshotPolicy gets the snapshot policy with the given name
func GetSnapshotPolicy(ctx context.Context, service *OsClusterClient, policyName string) (*responses.GetSnapshotPolicyResponse, error) {
	resp, err := service.GetSnapshotPolicyConfig(ctx, policyName)
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
	snapshotPolicyResponse := responses.GetSnapshotPolicyResponse{}
	if resp != nil && resp.Body != nil {
		err := json.NewDecoder(resp.Body).Decode(&snapshotPolicyResponse)
		if err != nil {
			return nil, err
		}
		return &snapshotPolicyResponse, nil
	}
	return nil, fmt.Errorf("response is empty")
}

// CreateSnapshotPolicy creates the passed policy
func CreateSnapshotPolicy(ctx context.Context, service *OsClusterClient, snapshotpolicy requests.SnapshotPolicy, policyName string) error {
	spec := opensearchutil.NewJSONReader(snapshotpolicy)
	resp, err := service.CreateSnapshotPolicyConfig(ctx, policyName, spec)
	if err != nil {
		return err
	}
	defer helpers.SafeClose(resp.Body)
	if resp.IsError() {
		return fmt.Errorf("failed to create snapshot policy: %s", resp.String())
	}
	return nil
}

// DeleteSnapshotPolicy deletes the given policy
func DeleteSnapshotPolicy(ctx context.Context, service *OsClusterClient, policyName string) error {
	resp, err := service.DeleteSnapshotPolicyConfig(ctx, policyName)
	if err != nil {
		return err
	}
	defer helpers.SafeClose(resp.Body)
	if resp.IsError() {
		return fmt.Errorf("failed to delete snapshot policy: %s", resp.String())
	}
	return nil
}

// UpdateSnapshotPolicy updates the given policy
func UpdateSnapshotPolicy(ctx context.Context, service *OsClusterClient, snapshotpolicy requests.SnapshotPolicy, seqno, primterm *int, policyName string) error {
	spec := opensearchutil.NewJSONReader(snapshotpolicy)
	resp, err := service.UpdateSnapshotPolicyConfig(ctx, policyName, *seqno, *primterm, spec)
	if err != nil {
		return err
	}
	defer helpers.SafeClose(resp.Body)
	if resp.IsError() {
		return fmt.Errorf("failed to update snapshot policy: %s", resp.String())
	}
	return nil
}
