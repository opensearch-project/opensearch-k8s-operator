package services

import (
	"errors"
	"fmt"
)

var (
	ErrClusterHealthOperation = errors.New("cluster health failed")
)

func ErrClusterHealthGetFailed(resp string) error {
	return fmt.Errorf("get error %w: %s", ErrClusterHealthOperation, resp)
}
