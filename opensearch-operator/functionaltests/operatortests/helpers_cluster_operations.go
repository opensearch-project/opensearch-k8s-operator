package operatortests

import (
	"context"
	"fmt"
	"time"

	opsterv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterOperations provides helper functions for cluster operations
type ClusterOperations struct {
	k8sClient client.Client
	namespace string
}

// NewClusterOperations creates a new cluster operations helper
func NewClusterOperations(k8sClient client.Client, namespace string) *ClusterOperations {
	return &ClusterOperations{
		k8sClient: k8sClient,
		namespace: namespace,
	}
}

// UpgradeCluster upgrades the cluster to the specified version
func (co *ClusterOperations) UpgradeCluster(clusterName string, opensearchVersion, dashboardsVersion string) error {
	cluster := unstructured.Unstructured{}
	cluster.SetGroupVersionKind(schema.GroupVersionKind{Group: "opensearch.opster.io", Version: "v1", Kind: "OpenSearchCluster"})

	err := co.k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: co.namespace}, &cluster)
	if err != nil {
		return err
	}

	SetNestedKey(cluster.Object, opensearchVersion, "spec", "general", "version")
	SetNestedKey(cluster.Object, dashboardsVersion, "spec", "dashboards", "version")

	return co.k8sClient.Update(context.Background(), &cluster)
}

// ScaleNodePool scales a node pool to the specified number of replicas
func (co *ClusterOperations) ScaleNodePool(clusterName, componentName string, replicas int32) error {
	cluster := &opsterv1.OpenSearchCluster{}
	err := co.k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: co.namespace}, cluster)
	if err != nil {
		return err
	}

	// Find and update the node pool
	for i := range cluster.Spec.NodePools {
		if cluster.Spec.NodePools[i].Component == componentName {
			cluster.Spec.NodePools[i].Replicas = replicas
			return co.k8sClient.Update(context.Background(), cluster)
		}
	}

	return fmt.Errorf("node pool %s not found", componentName)
}

// AddNodePool adds a new node pool to the cluster
func (co *ClusterOperations) AddNodePool(clusterName string, nodePool opsterv1.NodePool) error {
	cluster := &opsterv1.OpenSearchCluster{}
	err := co.k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: co.namespace}, cluster)
	if err != nil {
		return err
	}

	// Check if node pool already exists
	for _, np := range cluster.Spec.NodePools {
		if np.Component == nodePool.Component {
			return fmt.Errorf("node pool %s already exists", nodePool.Component)
		}
	}

	cluster.Spec.NodePools = append(cluster.Spec.NodePools, nodePool)
	return co.k8sClient.Update(context.Background(), cluster)
}

// RemoveNodePool removes a node pool from the cluster
func (co *ClusterOperations) RemoveNodePool(clusterName, componentName string) error {
	cluster := &opsterv1.OpenSearchCluster{}
	err := co.k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: co.namespace}, cluster)
	if err != nil {
		return err
	}

	// Find and remove the node pool
	newNodePools := []opsterv1.NodePool{}
	found := false
	for _, np := range cluster.Spec.NodePools {
		if np.Component == componentName {
			found = true
			continue
		}
		newNodePools = append(newNodePools, np)
	}

	if !found {
		return fmt.Errorf("node pool %s not found", componentName)
	}

	cluster.Spec.NodePools = newNodePools
	return co.k8sClient.Update(context.Background(), cluster)
}

// WaitForNodePoolReady waits for a node pool to be ready
func (co *ClusterOperations) WaitForNodePoolReady(clusterName, componentName string, expectedReplicas int32, timeout time.Duration) error {
	stsName := clusterName + "-" + componentName
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for node pool %s to be ready", componentName)
		case <-ticker.C:
			sts := appsv1.StatefulSet{}
			err := co.k8sClient.Get(context.Background(), client.ObjectKey{Name: stsName, Namespace: co.namespace}, &sts)
			if err != nil {
				continue
			}
			if sts.Status.ReadyReplicas == expectedReplicas && sts.Status.UpdatedReplicas == expectedReplicas {
				return nil
			}
		}
	}
}

// WaitForUpgradeComplete waits for cluster upgrade to complete
func (co *ClusterOperations) WaitForUpgradeComplete(clusterName string, expectedImage string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for upgrade to complete")
		case <-ticker.C:
			sts := appsv1.StatefulSet{}
			err := co.k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-masters", Namespace: co.namespace}, &sts)
			if err != nil {
				continue
			}

			// Check if image matches expected version
			if len(sts.Spec.Template.Spec.Containers) == 0 {
				continue
			}

			actualImage := sts.Spec.Template.Spec.Containers[0].Image
			if actualImage != expectedImage {
				continue
			}

			// Check if all replicas are updated and ready
			if sts.Status.UpdatedReplicas == sts.Status.Replicas &&
				sts.Status.ReadyReplicas == sts.Status.Replicas {
				return nil
			}
		}
	}
}

// WaitForDashboardsReady waits for dashboards to be ready
func (co *ClusterOperations) WaitForDashboardsReady(clusterName string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for dashboards to be ready")
		case <-ticker.C:
			deployment := appsv1.Deployment{}
			err := co.k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-dashboards", Namespace: co.namespace}, &deployment)
			if err != nil {
				continue
			}
			if deployment.Status.ReadyReplicas > 0 {
				return nil
			}
		}
	}
}

// GetNodePoolReplicas returns the current number of replicas for a node pool
func (co *ClusterOperations) GetNodePoolReplicas(clusterName, componentName string) (int32, error) {
	cluster := &opsterv1.OpenSearchCluster{}
	err := co.k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: co.namespace}, cluster)
	if err != nil {
		return 0, err
	}

	for _, np := range cluster.Spec.NodePools {
		if np.Component == componentName {
			return np.Replicas, nil
		}
	}

	return 0, fmt.Errorf("node pool %s not found", componentName)
}

// GetClusterVersion returns the current OpenSearch version
func (co *ClusterOperations) GetClusterVersion(clusterName string) (string, error) {
	cluster := &opsterv1.OpenSearchCluster{}
	err := co.k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: co.namespace}, cluster)
	if err != nil {
		return "", err
	}

	return cluster.Spec.General.Version, nil
}
