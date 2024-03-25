package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"time"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	"github.com/cisco-open/operator-tools/pkg/reconciler"
	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type SnapshotRepositoryReconciler struct {
	client k8s.K8sClient
	ReconcilerOptions
	ctx      context.Context
	osClient *services.OsClusterClient
	recorder record.EventRecorder
	instance *opsterv1.OpenSearchCluster
	logger   logr.Logger
}

func NewSnapshotRepositoryReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	instance *opsterv1.OpenSearchCluster,
	opts ...ReconcilerOption,
) *SnapshotRepositoryReconciler {
	options := ReconcilerOptions{}
	options.apply(opts...)
	return &SnapshotRepositoryReconciler{
		client:            k8s.NewK8sClient(client, ctx, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "snapshot_repository"))),
		ReconcilerOptions: options,
		ctx:               ctx,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx).WithValues("reconciler", "snapshot_repository"),
	}
}

func (r *SnapshotRepositoryReconciler) Reconcile() (ctrl.Result, error) {
	if r.instance.Spec.General.SnapshotRepositories == nil || len(r.instance.Spec.General.SnapshotRepositories) == 0 {
		// Skip reconcile if no repositories are configured
		return ctrl.Result{}, nil
	}
	var reason string
	var retErr error

	// Check cluster is ready
	if r.instance.Status.Phase != opsterv1.PhaseRunning {
		r.logger.Info("opensearch cluster is not running, requeueing")
		reason = "waiting for opensearch cluster status to be running"
		r.recorder.Event(r.instance, "Normal", opensearchPending, reason)
		retResult := ctrl.Result{
			Requeue:      true,
			RequeueAfter: 10 * time.Second,
		}
		return retResult, nil
	}

	r.osClient, retErr = util.CreateClientForCluster(r.client, r.ctx, r.instance, r.osClientTransport)
	if retErr != nil {
		reason = "error creating opensearch client"
		r.recorder.Event(r.instance, "Warning", opensearchError, reason)
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, retErr
	}

	var lastErr error
	// Go through configured repositories and reconcile them
	for _, snapshotRepository := range r.instance.Spec.General.SnapshotRepositories {
		err := r.ReconcileRepository(&snapshotRepository)
		if err != nil {
			lastErr = err
		}
	}
	// Requeue if reconcile failed for at least one repository
	if lastErr != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, lastErr
	} else {
		return ctrl.Result{}, nil
	}
}

func (r *SnapshotRepositoryReconciler) ReconcileRepository(repoConfig *opsterv1.SnapshotRepoConfig) error {
	newSnapshotRepository := mapSnapshotRepository(repoConfig)

	existingRepository, retErr := services.GetSnapshotRepository(r.ctx, r.osClient, repoConfig.Name)
	if retErr != nil && retErr != services.ErrRepoNotFound {
		reason := "failed to get snapshot repository from Opensearch API"
		r.logger.Error(retErr, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
		return retErr
	}
	if errors.Is(retErr, services.ErrRepoNotFound) {
		// create new snapshot repository
		r.logger.V(1).Info(fmt.Sprintf("snapshot repository %s not found, creating.", repoConfig.Name))
		retErr = services.CreateSnapshotRepository(r.ctx, r.osClient, repoConfig.Name, newSnapshotRepository)
		if retErr != nil {
			reason := "failed to create snapshot repository"
			r.logger.Error(retErr, reason)
			r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
			return retErr
		}
		r.recorder.Event(r.instance, "Normal", opensearchAPIUpdated, "snapshot repository created in opensearch")
	} else {
		// update existing if needed
		shouldUpdate, err := services.ShouldUpdateSnapshotRepository(r.ctx, newSnapshotRepository, *existingRepository)
		if err != nil {
			reason := "failed to compare snapshot repository for changes"
			r.logger.Error(retErr, reason)
			r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
			return err
		}
		if shouldUpdate {
			err := services.UpdateSnapshotRepository(r.ctx, r.osClient, repoConfig.Name, newSnapshotRepository)
			if err != nil {
				reason := "failed to update snapshot repository"
				r.logger.Error(retErr, reason)
				r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
			} else {
				r.recorder.Event(r.instance, "Normal", opensearchAPIUpdated, "snapshot repository updated in opensearch")
			}
			return err
		}
	}
	return nil
}

func (r *SnapshotRepositoryReconciler) Delete() error {
	// this is only called if the entire cluster is deleted, no need to explictly delete the snapshot repositories
	return nil
}

func mapSnapshotRepository(repository *opsterv1.SnapshotRepoConfig) requests.SnapshotRepository {
	return requests.SnapshotRepository{
		Type:     repository.Type,
		Settings: repository.Settings,
	}
}
