package reconcilers

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/client-go/tools/record"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"
	opsterv1 "opensearch.opster.io/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	opensearchPending     = "OpensearchPending"
	opensearchError       = "OpensearchError"
	opensearchAPIError    = "OpensearchAPIError"
	opensearchRefMismatch = "OpensearchRefMismatch"
	opensearchAPIUpdated  = "OpensearchAPIUpdated"
	passwordError         = "PasswordError"
	statusError           = "StatusUpdateError"
)

type ComponentReconciler func() (reconcile.Result, error)

type ReconcilerOptions struct {
	osClientTransport http.RoundTripper
	updateStatus      *bool
}

type ReconcilerOption func(*ReconcilerOptions)

func (o *ReconcilerOptions) apply(opts ...ReconcilerOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithOSClientTransport(transport http.RoundTripper) ReconcilerOption {
	return func(o *ReconcilerOptions) {
		o.osClientTransport = transport
	}
}

func WithUpdateStatus(update bool) ReconcilerOption {
	return func(o *ReconcilerOptions) {
		o.updateStatus = &update
	}
}

type ReconcilerContext struct {
	Volumes          []corev1.Volume
	VolumeMounts     []corev1.VolumeMount
	NodePoolHashes   []NodePoolHash
	DashboardsConfig map[string]string
	OpenSearchConfig map[string]string
	recorder         record.EventRecorder
	instance         *opsterv1.OpenSearchCluster
}

type NodePoolHash struct {
	Component  string
	ConfigHash string
}

func NewReconcilerContext(recorder record.EventRecorder, instance *opsterv1.OpenSearchCluster, nodepools []opsterv1.NodePool) ReconcilerContext {
	var nodePoolHashes []NodePoolHash
	for _, nodepool := range nodepools {
		nodePoolHashes = append(nodePoolHashes, NodePoolHash{
			Component: nodepool.Component,
		})
	}
	return ReconcilerContext{
		NodePoolHashes:   nodePoolHashes,
		OpenSearchConfig: make(map[string]string),
		DashboardsConfig: make(map[string]string),
		recorder:         recorder,
		instance:         instance,
	}
}

func (c *ReconcilerContext) AddConfig(key string, value string) {
	_, exists := c.OpenSearchConfig[key]
	if exists {
		fmt.Printf("Warning: Config key '%s' already exists. Will be overwritten\n", key)
		c.recorder.Eventf(c.instance, "Warning", "ConfigDuplicateKey", "Config key '%s' already exists in opensearch config. Will be overwritten", key)
	}
	c.OpenSearchConfig[key] = value
}

func (c *ReconcilerContext) AddDashboardsConfig(key string, value string) {
	_, exists := c.DashboardsConfig[key]
	if exists {
		fmt.Printf("Warning: Dashboards Config key '%s' already exists. Will be overwritten\n", key)
		c.recorder.Eventf(c.instance, "Warning", "ConfigDuplicateKey", "Config key '%s' already exists in dashboards config. Will be overwritten", key)
	}
	c.DashboardsConfig[key] = value

}

// fetchNodePoolHash gets the hash of the config for a specific node pool
func (c *ReconcilerContext) fetchNodePoolHash(name string) (bool, NodePoolHash) {
	for _, config := range c.NodePoolHashes {
		if config.Component == name {
			return true, config
		}
	}
	return false, NodePoolHash{}
}

// replaceNodePoolHash updates the hash of the config for a specific node pool
func (c *ReconcilerContext) replaceNodePoolHash(newConfig NodePoolHash) {
	var configs []NodePoolHash
	for _, config := range c.NodePoolHashes {
		if config.Component == newConfig.Component {
			configs = append(configs, newConfig)
		} else {
			configs = append(configs, config)
		}
	}
	c.NodePoolHashes = configs
}

func UpdateComponentStatus(
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
