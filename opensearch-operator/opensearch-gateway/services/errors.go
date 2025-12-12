package services

import (
	"errors"
	"fmt"
)

var (
	ErrClusterAllocationExplainOperation = errors.New("cluster allocation explain failed")
	ErrClusterHealthOperation            = errors.New("cluster health failed")
	ErrClusterSettingsOperation          = errors.New("cluster settings failed")
	ErrCatIndicesOperation               = errors.New("cat indices failed")
)

func ErrClusterAllocationExplainGetFailed(resp string) error {
	return fmt.Errorf("get error %w: %s", ErrClusterAllocationExplainOperation, resp)
}

func ErrClusterHealthGetFailed(resp string) error {
	return fmt.Errorf("get error %w: %s", ErrClusterHealthOperation, resp)
}

func ErrClusterSettingsGetFailed(resp string) error {
	return fmt.Errorf("get error %w: %s", ErrClusterSettingsOperation, resp)
}

func ErrClusterSettingsPutFailed(resp string) error {
	return fmt.Errorf("put error %w: %s", ErrClusterSettingsOperation, resp)
}

func ErrCatIndicesFailed(resp string) error {
	return fmt.Errorf("%w: %s", ErrCatIndicesOperation, resp)
}
