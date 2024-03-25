package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/opensearch-project/opensearch-go/opensearchutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var ErrRepoNotFound = errors.New("snapshotRepository not found")

// checks if the passed SnapshotRepository is same as existing or needs update
func ShouldUpdateSnapshotRepository(ctx context.Context, newRepository, existingRepository requests.SnapshotRepository) (bool, error) {
	if cmp.Equal(newRepository, existingRepository, cmpopts.EquateEmpty()) {
		return false, nil
	}
	lg := log.FromContext(ctx).WithValues("os_service", "snapshotrepository")
	lg.V(1).Info(fmt.Sprintf("existing SnapshotRepository: %+v", existingRepository))
	lg.V(1).Info(fmt.Sprintf("new SnapshotRepository: %+v", newRepository))
	lg.Info("snapshotRepository exists and requires update")
	return true, nil
}

// checks if the snapshot repository with the given name already exists
func SnapshotRepositoryExists(ctx context.Context, service *OsClusterClient, repositoryName string) (bool, error) {
	resp, err := service.GetSnapshotRepository(ctx, repositoryName)
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

// fetches the snapshot repository with the given name
func GetSnapshotRepository(ctx context.Context, service *OsClusterClient, repositoryName string) (*requests.SnapshotRepository, error) {
	resp, err := service.GetSnapshotRepository(ctx, repositoryName)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, ErrRepoNotFound
	} else if resp.IsError() {
		return nil, fmt.Errorf("response from API is %s", resp.Status())
	}
	repoResponse := responses.SnapshotRepositoryResponse{}
	if resp != nil && resp.Body != nil {
		err := json.NewDecoder(resp.Body).Decode(&repoResponse)
		if err != nil {
			return nil, err
		}
		// the opensearch api returns a map of name -> repo config, so we extract the one for the repo we need
		repo, exists := repoResponse[repositoryName]
		if !exists {
			return nil, ErrRepoNotFound
		}
		return &repo, nil
	}
	return nil, fmt.Errorf("response is empty")
}

// creates the given SnapshotRepository
func CreateSnapshotRepository(ctx context.Context, service *OsClusterClient, repositoryName string, repository requests.SnapshotRepository) error {
	spec := opensearchutil.NewJSONReader(repository)
	resp, err := service.CreateSnapshotRepository(ctx, repositoryName, spec)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("failed to create snapshot repository: %s", resp.String())
	}
	return nil
}

// updates the given SnapshotRepository
func UpdateSnapshotRepository(ctx context.Context, service *OsClusterClient, repositoryName string, repository requests.SnapshotRepository) error {
	spec := opensearchutil.NewJSONReader(repository)
	resp, err := service.UpdateSnapshotRepository(ctx, repositoryName, spec)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("failed to update snapshot repository: %s", resp.String())
	}
	return nil
}

// deletes the given SnapshotRepository
func DeleteSnapshotRepository(ctx context.Context, service *OsClusterClient, repositoryName string) error {
	resp, err := service.DeleteSnapshotRepository(ctx, repositoryName)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("failed to delete snapshot repository: %s", resp.String())
	}
	return nil
}
