package services

import (
	"errors"
	"fmt"
)

var (
	ErrClusterHealthOperation   = errors.New("cluster health failed")
	ErrClusterSettingsOperation = errors.New("cluster settings failed")
	ErrCatIndicesOperation      = errors.New("cat indices failed")
)

func ErrClusterHealthGetFailed(resp string) error {
	return fmt.Errorf("get error %w: %s", ErrClusterHealthOperation, resp)
}

func ErrClusterSettingsGetFailed(resp string) error {
	return fmt.Errorf("get error %w: %s", ErrClusterSettingsOperation, resp)
}

func ErrCatIndicesFailed(resp string) error {
	return fmt.Errorf("%w: %s", ErrCatIndicesOperation, resp)
}
