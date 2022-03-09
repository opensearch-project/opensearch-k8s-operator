package reconcilers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"
	opsterv1 "opensearch.opster.io/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ComponentReconciler func() (reconcile.Result, error)

type ReconcilerContext struct {
	Volumes          []corev1.Volume
	VolumeMounts     []corev1.VolumeMount
	OpenSearchConfig map[string]string
	DashboardsConfig map[string]string
}

func NewReconcilerContext() ReconcilerContext {
	return ReconcilerContext{OpenSearchConfig: make(map[string]string), DashboardsConfig: make(map[string]string)}
}

func (c *ReconcilerContext) AddConfig(key string, value string) {
	_, exists := c.OpenSearchConfig[key]
	if exists {
		fmt.Printf("Warning: Config key '%s' already exists. Will be overwritten\n", key)
	}
	c.OpenSearchConfig[key] = value
}

func (c *ReconcilerContext) AddDashboardsConfig(key string, value string) {
	_, exists := c.DashboardsConfig[key]
	if exists {
		fmt.Printf("Warning: Config key '%s' already exists. Will be overwritten\n", key)
	}
	c.DashboardsConfig[key] = value
}

func UpdateOpensearchStatus(
	ctx context.Context,
	k8sClient client.Client,
	instance *opsterv1.OpenSearchCluster,
	status *opsterv1.ComponentStatus,
) error {
	if status != nil {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(instance), instance); err != nil {
				return err
			}
			found := false
			for idx, value := range instance.Status.ComponentsStatus {
				if value.Component == status.Component {
					instance.Status.ComponentsStatus[idx] = *status
					found = true
					break
				}
			}
			if !found {
				instance.Status.ComponentsStatus = append(instance.Status.ComponentsStatus, *status)
			}
			return k8sClient.Status().Update(ctx, instance)
		})
	}
	return nil
}
