package helpers

import (
	"context"
	"errors"
	"fmt"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sort"
	"time"

	"github.com/hashicorp/go-version"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	opsterv1 "opensearch.opster.io/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	stsUpdateWaitTime = 30
	updateStepTime    = 3
)

func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false

}

func GetField(v *appsv1.StatefulSetSpec, field string) interface{} {

	r := reflect.ValueOf(v)
	f := reflect.Indirect(r).FieldByName(field).Interface()
	return f
}

func RemoveIt(ss opsterv1.ComponentStatus, ssSlice []opsterv1.ComponentStatus) []opsterv1.ComponentStatus {
	for idx, v := range ssSlice {
		if v == ss {
			return append(ssSlice[0:idx], ssSlice[idx+1:]...)
		}
	}
	return ssSlice
}
func Replace(remove opsterv1.ComponentStatus, add opsterv1.ComponentStatus, ssSlice []opsterv1.ComponentStatus) []opsterv1.ComponentStatus {
	removedSlice := RemoveIt(remove, ssSlice)
	fullSliced := append(removedSlice, add)
	return fullSliced
}

func FindFirstPartial(
	arr []opsterv1.ComponentStatus,
	item opsterv1.ComponentStatus,
	predicator func(opsterv1.ComponentStatus, opsterv1.ComponentStatus) (opsterv1.ComponentStatus, bool),
) (opsterv1.ComponentStatus, bool) {
	for i := 0; i < len(arr); i++ {
		itemInArr, found := predicator(arr[i], item)
		if found {
			return itemInArr, found
		}
	}
	return item, false
}

func FindByPath(obj interface{}, keys []string) (interface{}, bool) {
	mobj, ok := obj.(map[string]interface{})
	if !ok {
		return nil, false
	}
	for i := 0; i < len(keys)-1; i++ {
		if currentVal, found := mobj[keys[i]]; found {
			subPath, ok := currentVal.(map[string]interface{})
			if !ok {
				return nil, false
			}
			mobj = subPath
		}
	}
	val, ok := mobj[keys[len(keys)-1]]
	return val, ok
}

func UsernameAndPassword(ctx context.Context, k8sClient client.Client, cr *opsterv1.OpenSearchCluster) (string, string, error) {
	if cr.Spec.Security != nil && cr.Spec.Security.Config != nil && cr.Spec.Security.Config.AdminCredentialsSecret.Name != "" {
		// Read credentials from secret
		credentialsSecret := corev1.Secret{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: cr.Spec.Security.Config.AdminCredentialsSecret.Name, Namespace: cr.Namespace}, &credentialsSecret); err != nil {
			return "", "", err
		}
		username, usernameExists := credentialsSecret.Data["username"]
		password, passwordExists := credentialsSecret.Data["password"]
		if !usernameExists || !passwordExists {
			return "", "", errors.New("username or password field missing")
		}
		return string(username), string(password), nil
	} else {
		// Use default demo credentials
		return "admin", "admin", nil
	}
}

func GetByDescriptionAndGroup(left opsterv1.ComponentStatus, right opsterv1.ComponentStatus) (opsterv1.ComponentStatus, bool) {
	if left.Description == right.Description && left.Component == right.Component {
		return left, true
	}
	return right, false
}

func MergeConfigs(left map[string]string, right map[string]string) map[string]string {
	if left == nil {
		return right
	}
	for k, v := range right {
		left[k] = v
	}
	return left
}

// Return the keys of the input map in sorted order
// Can be used if you want to iterate over a map but have a stable order
func SortedKeys(input map[string]string) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func ResolveClusterManagerRole(ver string) string {
	masterRole := "master"
	osVer, err := version.NewVersion(ver)

	clusterManagerVer, _ := version.NewVersion("2.0.0")
	if err == nil && osVer.GreaterThanOrEqual(clusterManagerVer) {
		masterRole = "cluster_manager"
	}
	return masterRole
}

// Map any cluster roles that have changed between major OpenSearch versions
func MapClusterRole(role string, ver string) string {
	osVer, err := version.NewVersion(ver)
	if err != nil {
		return role
	}
	clusterManagerVer, _ := version.NewVersion("2.0.0")
	is2XVersion := osVer.GreaterThanOrEqual(clusterManagerVer)
	if role == "master" && is2XVersion {
		return "cluster_manager"
	} else if role == "cluster_manager" && !is2XVersion {
		return "master"
	} else {
		return role
	}
}

func MapClusterRoles(roles []string, version string) []string {
	mapped_roles := []string{}
	for _, role := range roles {
		mapped_roles = append(mapped_roles, MapClusterRole(role, version))
	}
	return mapped_roles
}

// Get leftSlice strings not in rightSlice
func DiffSlice(leftSlice, rightSlice []string) []string {
	//diff := []string{}
	var diff []string

	for _, leftSliceString := range leftSlice {
		if !ContainsString(rightSlice, leftSliceString) {
			diff = append(diff, leftSliceString)
		}
	}
	return diff
}

// Count the number of PVCs created for the given NodePool
func CountPVCsForNodePool(ctx context.Context, k8sClient client.Client, cr *opsterv1.OpenSearchCluster, nodePool *opsterv1.NodePool) (int, error) {
	clusterReq, err := labels.NewRequirement(ClusterLabel, selection.Equals, []string{cr.ObjectMeta.Name})
	if err != nil {
		return 0, err
	}
	componentReq, err := labels.NewRequirement(NodePoolLabel, selection.Equals, []string{nodePool.Component})
	if err != nil {
		return 0, err
	}
	selector := labels.NewSelector()
	selector = selector.Add(*clusterReq, *componentReq)
	list := corev1.PersistentVolumeClaimList{}
	if err := k8sClient.List(ctx, &list, &client.ListOptions{LabelSelector: selector}); err != nil {
		return 0, err
	}
	return len(list.Items), nil
}

// Delete a STS with cascade=orphan and wait until it is actually deleted from the kubernetes API
func WaitForSTSDelete(ctx context.Context, k8sClient client.Client, obj *appsv1.StatefulSet) error {
	opts := client.DeleteOptions{}
	client.PropagationPolicy(metav1.DeletePropagationOrphan).ApplyToDelete(&opts)
	if err := k8sClient.Delete(ctx, obj, &opts); err != nil {
		return err
	}
	for i := 1; i <= stsUpdateWaitTime/updateStepTime; i++ {
		existing := appsv1.StatefulSet{}
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), &existing)
		if err != nil {
			return nil
		}
		time.Sleep(time.Second * updateStepTime)
	}
	return fmt.Errorf("failed to delete STS")
}

// Wait for max 30s until a STS has at least the given number of replicas
func WaitForSTSReplicas(ctx context.Context, k8sClient client.Client, obj *appsv1.StatefulSet, replicas int32) error {
	for i := 1; i <= stsUpdateWaitTime/updateStepTime; i++ {
		existing := appsv1.StatefulSet{}
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), &existing)
		if err == nil {
			if existing.Status.Replicas >= replicas {
				return nil
			}
		}
		time.Sleep(time.Second * updateStepTime)
	}
	return fmt.Errorf("failed to wait for replicas")
}

// Wait for max 30s until a STS has a normal status (CurrentRevision != "")
func WaitForSTSStatus(ctx context.Context, k8sClient client.Client, obj *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	for i := 1; i <= stsUpdateWaitTime/updateStepTime; i++ {
		existing := appsv1.StatefulSet{}
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), &existing)
		if err == nil {
			if existing.Status.CurrentRevision != "" {
				return &existing, nil
			}
		}
		time.Sleep(time.Second * updateStepTime)
	}
	return nil, fmt.Errorf("failed to wait for STS")
}

// GetSTSForNodePool returns the corresponding sts for a given nodePool and cluster name
func GetSTSForNodePool(ctx context.Context, k8sClient client.Client, nodePool opsterv1.NodePool, clusterName, clusterNamespace string) (*appsv1.StatefulSet, error) {
	sts := &appsv1.StatefulSet{}
	stsName := clusterName + "-" + nodePool.Component

	err := k8sClient.Get(ctx, types.NamespacedName{Name: stsName, Namespace: clusterNamespace}, sts)

	return sts, err
}

// DeleteSTSForNodePool deletes the sts for the corresponding nodePool
func DeleteSTSForNodePool(ctx context.Context, k8sClient client.Client, nodePool opsterv1.NodePool, clusterName, clusterNamespace string) error {

	sts, err := GetSTSForNodePool(ctx, k8sClient, nodePool, clusterName, clusterNamespace)
	if err != nil {
		return err
	}

	opts := client.DeleteOptions{}
	// Add this so pods of the sts are deleted as well, otherwise they would remain as orphaned pods
	client.PropagationPolicy(metav1.DeletePropagationForeground).ApplyToDelete(&opts)

	err = k8sClient.Delete(ctx, sts, &opts)

	return err
}

// DeleteSecurityUpdateJob deletes the securityconfig update job
func DeleteSecurityUpdateJob(ctx context.Context, k8sClient client.Client, clusterName, clusterNamespace string) error {
	jobName := clusterName + "-securityconfig-update"
	job := batchv1.Job{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: jobName, Namespace: clusterNamespace}, &job)

	if err != nil {
		return err
	}

	opts := client.DeleteOptions{}
	// Add this so pods of the job are deleted as well, otherwise they would remain as orphaned pods
	client.PropagationPolicy(metav1.DeletePropagationForeground).ApplyToDelete(&opts)
	err = k8sClient.Delete(ctx, &job, &opts)

	return err
}

func HasDataRole(nodePool *opsterv1.NodePool) bool {
	return ContainsString(nodePool.Roles, "data")
}

func HasManagerRole(nodePool *opsterv1.NodePool) bool {
	return ContainsString(nodePool.Roles, "master") || ContainsString(nodePool.Roles, "cluster_manager")
}

func RemoveDuplicateStrings(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

// Compares whether v1 is LessThan v2
func CompareVersions(v1 string, v2 string) bool {
	ver1, err := version.NewVersion(v1)
	ver2, _ := version.NewVersion(v2)
	return err == nil && ver1.LessThan(ver2)
}
