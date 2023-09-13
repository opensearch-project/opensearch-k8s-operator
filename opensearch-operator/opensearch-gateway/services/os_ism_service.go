package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"reflect"

	"github.com/opensearch-project/opensearch-go/opensearchutil"
	"opensearch.opster.io/opensearch-gateway/requests"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ShouldUpdateISMPolicy checks if the passed policy is same as existing or needs update
func ShouldUpdateISMPolicy(ctx context.Context, newPolicy, existingPolicy requests.Policy) (bool, error) {
	if reflect.DeepEqual(newPolicy, existingPolicy) {
		return false, nil
	}
	lg := log.FromContext(ctx).WithValues("os_service", "policy")
	lg.V(1).Info(fmt.Sprintf("existing policy: %+v", existingPolicy))
	lg.V(1).Info(fmt.Sprintf("new policy: %+v", newPolicy))
	lg.Info("policy exists and requires update")
	return true, nil
}

// PolicyExists checks if the passed policy already exists or not
func PolicyExists(ctx context.Context, service *OsClusterClient, policyName string) (bool, error) {
	resp, err := service.GetISMConfig(ctx, policyName)
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

// GetPolicy fetches the passed policy
func GetPolicy(ctx context.Context, service *OsClusterClient, policyName string) (*opensearchapi.Response, error) {
	resp, err := service.GetISMConfig(ctx, policyName)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// CreateISMPolicy creates the passed policy
func CreateISMPolicy(ctx context.Context, service *OsClusterClient, ismpolicy requests.Policy, policyId string) error {
	spec := opensearchutil.NewJSONReader(ismpolicy)
	resp, err := service.PutISMConfig(ctx, policyId, spec)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("failed to create ism policy: %s", resp.String())
	}
	return nil
}

// UpdateISMPolicy updates the given policy
func UpdateISMPolicy(ctx context.Context, service *OsClusterClient, ismpolicy requests.Policy, seqno, primterm *int, policyName string) error {
	spec := opensearchutil.NewJSONReader(ismpolicy)
	resp, err := service.UpdateISMConfig(ctx, policyName, *seqno, *primterm, spec)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("Failed to create ism policy: %s", resp.String())
	}
	return nil
}

// DeleteISMPolicy deletes the given policy
func DeleteISMPolicy(ctx context.Context, service *OsClusterClient, policyName string) error {
	resp, err := service.DeleteISMConfig(ctx, policyName)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("Failed to delete ism policy: %s", resp.String())
	}
	return nil
}

// GetPolicyFromResponse extracts the policy from the response
func GetPolicyFromResponse(ctx context.Context, resp *opensearchapi.Response) (*requests.Policy, error) {
	ismResponse := requests.Policy{}
	if resp != nil && resp.Body != nil {
		err := json.NewDecoder(resp.Body).Decode(&ismResponse)
		if err != nil {
			return nil, err
		}
		return &ismResponse, nil
	}
	return nil, fmt.Errorf("response is empty")
}
