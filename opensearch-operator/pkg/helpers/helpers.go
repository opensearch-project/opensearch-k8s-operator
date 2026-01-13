package helpers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/resource"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	policyv1 "k8s.io/api/policy/v1"

	version "github.com/hashicorp/go-version"
	opsterv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	stsUpdateWaitTime = 30
	updateStepTime    = 2

	stsRevisionLabel = "controller-revision-hash"

	// Default UID and GID for OpenSearch containers
	DefaultUID = int64(1000)
	DefaultGID = int64(1000)
)

type User struct {
	Hash         string   `yaml:"hash"`
	Reserved     bool     `yaml:"reserved"`
	BackendRoles []string `yaml:"backend_roles,omitempty"`
	Description  string   `yaml:"description"`
}

type Meta struct {
	Type          string `yaml:"type"`
	ConfigVersion int    `yaml:"config_version"`
}

type InternalUserConfig struct {
	Meta         Meta  `yaml:"_meta"`
	Admin        User  `yaml:"admin"`
	Kibanaserver *User `yaml:"kibanaserver,omitempty"`
}

func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// IsHttpTlsEnabled determines if HTTP TLS is enabled for the cluster.
// If enabled is nil (not set): enabled by default if HTTP config exists.
// If enabled is true: explicitly enabled.
// If enabled is false: explicitly disabled.
func IsHttpTlsEnabled(cluster *opsterv1.OpenSearchCluster) bool {
	if cluster.Spec.Security == nil || cluster.Spec.Security.Tls == nil {
		return false
	}
	tlsConfig := cluster.Spec.Security.Tls.Http
	if tlsConfig == nil {
		return false
	}
	// If explicitly set, use that value
	if tlsConfig.Enabled != nil {
		return *tlsConfig.Enabled
	}
	// Default: enabled if HTTP config is provided
	return true
}

func CheckVersionConstraint(cluster *opsterv1.OpenSearchCluster, constraint string, defaultOnError bool, errMsg string) bool {
	versionConstraint, err := semver.NewConstraint(constraint)
	if err != nil {
		panic(err)
	}

	version, err := semver.NewVersion(cluster.Spec.General.Version)
	if err != nil {
		log.Println(errMsg)
		return defaultOnError
	}
	return versionConstraint.Check(version)
}

func SecurityChangeVersion(cluster *opsterv1.OpenSearchCluster) bool {
	return CheckVersionConstraint(
		cluster,
		">=2.0.0",
		true,
		"unable to parse version, assuming >= 2.0.0",
	)
}

func SupportsHotReload(cluster *opsterv1.OpenSearchCluster) bool {
	return CheckVersionConstraint(
		cluster,
		">=2.19.1",
		false,
		"unable to parse version for hot reload check, assuming not supported",
	)
}

// IsTransportTlsEnabled determines if transport TLS should be enabled.
// If enabled is nil (not set): enabled by default if transport config exists.
// If enabled is true: explicitly enabled.
// If enabled is false: explicitly disabled.
func IsTransportTlsEnabled(cluster *opsterv1.OpenSearchCluster) bool {
	if cluster.Spec.Security == nil || cluster.Spec.Security.Tls == nil {
		return false
	}
	tlsConfig := cluster.Spec.Security.Tls.Transport
	if tlsConfig == nil {
		return false
	}
	// If explicitly set, use that value
	if tlsConfig.Enabled != nil {
		return *tlsConfig.Enabled
	}
	return true
}

func IsSecurityPluginEnabled(cr *opsterv1.OpenSearchCluster) bool {

	if SecurityChangeVersion(cr) {
		return IsHttpTlsEnabled(cr)
	}
	return IsTransportTlsEnabled(cr)
}

// ClusterURL returns the URL for communicating with the OpenSearch cluster.
// If OperatorClusterURL is specified, it uses that custom URL.
// Otherwise, it constructs the default internal Kubernetes service DNS name.
func ClusterURL(cluster *opsterv1.OpenSearchCluster) string {
	httpPort := cluster.Spec.General.HttpPort
	if httpPort == 0 {
		httpPort = 9200 // default port
	}

	protocol := "https"
	// Check if HTTP TLS is enabled
	if !IsHttpTlsEnabled(cluster) {
		protocol = "http"
	}

	if cluster.Spec.General.OperatorClusterURL != nil && *cluster.Spec.General.OperatorClusterURL != "" {
		return fmt.Sprintf("%s://%s:%d", protocol, *cluster.Spec.General.OperatorClusterURL, httpPort)
	}

	// Default internal Kubernetes service DNS name
	return fmt.Sprintf("%s://%s.%s.svc.%s:%d",
		protocol,
		cluster.Spec.General.ServiceName,
		cluster.Namespace,
		ClusterDnsBase(),
		httpPort,
	)
}

func GetField(v *appsv1.StatefulSetSpec, field string) interface{} {
	r := reflect.ValueOf(v)
	f := reflect.Indirect(r).FieldByName(field).Interface()
	return f
}

func RemoveIt(ss opsterv1.ComponentStatus, ssSlice []opsterv1.ComponentStatus) []opsterv1.ComponentStatus {
	for idx, v := range ssSlice {
		if ComponentStatusEqual(v, ss) {
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

func ComponentStatusEqual(left opsterv1.ComponentStatus, right opsterv1.ComponentStatus) bool {
	return left.Component == right.Component && left.Description == right.Description && left.Status == right.Status
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

func FindAllPartial(
	arr []opsterv1.ComponentStatus,
	item opsterv1.ComponentStatus,
	predicator func(opsterv1.ComponentStatus, opsterv1.ComponentStatus) (opsterv1.ComponentStatus, bool),
) []opsterv1.ComponentStatus {
	var result []opsterv1.ComponentStatus

	for i := 0; i < len(arr); i++ {
		itemInArr, found := predicator(arr[i], item)
		if found {
			result = append(result, itemInArr)
		}
	}
	return result
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

func EnsureAdminCredentialsSecret(k8sClient k8s.K8sClient, cr *opsterv1.OpenSearchCluster) (*corev1.Secret, bool, error) {
	// Check if user provided AdminCredentialsSecret via Security.Config
	if cr.Spec.Security != nil && cr.Spec.Security.Config != nil && cr.Spec.Security.Config.AdminCredentialsSecret.Name != "" {
		secret, err := k8sClient.GetSecret(cr.Spec.Security.Config.AdminCredentialsSecret.Name, cr.Namespace)
		return &secret, false, err
	}

	// Always generate/administer the admin credentials secret
	generatedName := GeneratedAdminCredentialsSecretName(cr)
	secret, err := k8sClient.GetSecret(generatedName, cr.Namespace)
	if err == nil {
		return &secret, true, nil
	}
	if !k8serrors.IsNotFound(err) {
		return nil, true, err
	}

	randomPassword := rand.Text()

	adminSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generatedName,
			Namespace: cr.Namespace,
		},
		StringData: map[string]string{
			"username": "admin",
			"password": randomPassword,
		},
	}
	if _, err := k8sClient.CreateSecret(adminSecret); err != nil {
		return nil, true, err
	}

	createdSecret, err := k8sClient.GetSecret(generatedName, cr.Namespace)
	if err != nil {
		return nil, true, err
	}
	return &createdSecret, true, nil
}

func BuildGeneratedSecurityConfigSecret(k8sClient k8s.K8sClient, cr *opsterv1.OpenSearchCluster, adminSecret *corev1.Secret) (*corev1.Secret, error) {
	baseData, err := defaultSecurityconfigData()
	if err != nil {
		return nil, err
	}

	if cr.Spec.Security != nil && cr.Spec.Security.Config != nil && cr.Spec.Security.Config.SecurityconfigSecret.Name != "" {
		userSecret, err := k8sClient.GetSecret(cr.Spec.Security.Config.SecurityconfigSecret.Name, cr.Namespace)
		if err != nil {
			return nil, err
		}
		for key, value := range userSecret.Data {
			baseData[key] = append([]byte(nil), value...)
		}
	}

	adminPassword, passwordExists := adminSecret.Data["password"]
	if !passwordExists {
		return nil, errors.New("admin credentials secret missing password field")
	}

	dashboardsSecret, _, err := EnsureDashboardsCredentialsSecret(k8sClient, cr)
	if err != nil {
		return nil, err
	}
	var dashboardsPassword []byte
	if dashboardsSecret != nil {
		if pwd, exists := dashboardsSecret.Data["password"]; exists {
			dashboardsPassword = pwd
		}
	}
	if len(dashboardsPassword) == 0 {
		return nil, errors.New("dashboards credentials secret missing password field")
	}

	internalUsers, ok := baseData["internal_users.yml"]
	if !ok {
		return nil, errors.New("securityconfig missing internal_users.yml")
	}

	generatedName := GeneratedSecurityConfigSecretName(cr)
	var existingGenerated *corev1.Secret
	existingSecret, err := k8sClient.GetSecret(generatedName, cr.Namespace)
	if err == nil {
		existingGenerated = &existingSecret
	} else if !k8serrors.IsNotFound(err) {
		return nil, err
	}

	var adminHashOverride, dashboardsHashOverride string
	if existingGenerated != nil {
		if existingInternal, exists := existingGenerated.Data["internal_users.yml"]; exists {
			var existingConfig InternalUserConfig
			if err := yaml.Unmarshal(existingInternal, &existingConfig); err == nil {
				if existingConfig.Admin.Hash != "" && bcrypt.CompareHashAndPassword([]byte(existingConfig.Admin.Hash), adminPassword) == nil {
					adminHashOverride = existingConfig.Admin.Hash
				}
				if existingConfig.Kibanaserver != nil && existingConfig.Kibanaserver.Hash != "" {
					if bcrypt.CompareHashAndPassword([]byte(existingConfig.Kibanaserver.Hash), dashboardsPassword) == nil {
						dashboardsHashOverride = existingConfig.Kibanaserver.Hash
					}
				}
			}
		}
	}

	internalUsers, err = applyUserHashes(internalUsers, adminPassword, adminHashOverride, dashboardsPassword, dashboardsHashOverride)
	if err != nil {
		return nil, err
	}
	baseData["internal_users.yml"] = internalUsers

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generatedName,
			Namespace: cr.Namespace,
		},
		Data: baseData,
		Type: corev1.SecretTypeOpaque,
	}

	return secret, nil
}

func applyUserHashes(internalUserData []byte, adminPassword []byte, adminHashOverride string, dashboardsPassword []byte, dashboardsHashOverride string) ([]byte, error) {
	var data InternalUserConfig
	if err := yaml.Unmarshal(internalUserData, &data); err != nil {
		return nil, err
	}

	var adminHash string
	if adminHashOverride != "" {
		adminHash = adminHashOverride
	} else {
		hashed, err := bcrypt.GenerateFromPassword(adminPassword, 12)
		if err != nil {
			return nil, err
		}
		adminHash = string(hashed)
	}
	data.Admin.Hash = adminHash

	if !data.Admin.Reserved {
		data.Admin.Reserved = true
	}
	if len(data.Admin.BackendRoles) == 0 {
		data.Admin.BackendRoles = []string{"admin"}
	} else {
		found := false
		for _, role := range data.Admin.BackendRoles {
			if role == "admin" {
				found = true
				break
			}
		}
		if !found {
			data.Admin.BackendRoles = append(data.Admin.BackendRoles, "admin")
		}
	}

	if data.Kibanaserver == nil {
		data.Kibanaserver = &User{}
	}

	var dashboardsHash string
	if dashboardsHashOverride != "" {
		dashboardsHash = dashboardsHashOverride
	} else {
		hashed, err := bcrypt.GenerateFromPassword(dashboardsPassword, 12)
		if err != nil {
			return nil, err
		}
		dashboardsHash = string(hashed)
	}
	data.Kibanaserver.Hash = dashboardsHash
	if !data.Kibanaserver.Reserved {
		data.Kibanaserver.Reserved = true
	}
	if data.Kibanaserver.Description == "" {
		data.Kibanaserver.Description = "Demo user for the OpenSearch Dashboards server"
	}

	modifiedYaml, err := yaml.Marshal(data)
	if err != nil {
		return nil, err
	}
	return modifiedYaml, nil
}

func UsernameAndPassword(k8sClient k8s.K8sClient, cr *opsterv1.OpenSearchCluster) (string, string, error) {
	// Check if user provided AdminCredentialsSecret via Security.Config
	var secretName string
	if cr.Spec.Security != nil && cr.Spec.Security.Config != nil && cr.Spec.Security.Config.AdminCredentialsSecret.Name != "" {
		secretName = cr.Spec.Security.Config.AdminCredentialsSecret.Name
	} else {
		secretName = GeneratedAdminCredentialsSecretName(cr)
	}

	credentialsSecret, err := k8sClient.GetSecret(secretName, cr.Namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) && secretName == GeneratedAdminCredentialsSecretName(cr) {
			credentialsSecretPtr, _, createErr := EnsureAdminCredentialsSecret(k8sClient, cr)
			if createErr != nil {
				return "", "", createErr
			}
			credentialsSecret = *credentialsSecretPtr
		} else {
			return "", "", err
		}
	}
	username, usernameExists := credentialsSecret.Data["username"]
	password, passwordExists := credentialsSecret.Data["password"]
	if !usernameExists || !passwordExists {
		return "", "", errors.New("username or password field missing")
	}
	return string(username), string(password), nil
}

func GetByDescriptionAndComponent(left opsterv1.ComponentStatus, right opsterv1.ComponentStatus) (opsterv1.ComponentStatus, bool) {
	if left.Description == right.Description && left.Component == right.Component {
		return left, true
	}
	return right, false
}

func GetByComponent(left opsterv1.ComponentStatus, right opsterv1.ComponentStatus) (opsterv1.ComponentStatus, bool) {
	if left.Component == right.Component {
		return left, true
	}
	return right, false
}

func MergeConfigs(left map[string]string, right map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range left {
		result[k] = v
	}
	for k, v := range right {
		result[k] = v
	}
	return result
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

// SortedJsonKeys helps to sort JSON object keys
// E.g. if API returns unsorted JSON object like this: {"resp": {"b": "2", "a": "1"}}
// this function could sort it and return {"resp": {"a": "1", "b": "2"}}
// This is useful for comparing Opensearch CRD objects and API responses
func SortedJsonKeys(obj *apiextensionsv1.JSON) (*apiextensionsv1.JSON, error) {
	m := make(map[string]interface{})
	if err := json.Unmarshal(obj.Raw, &m); err != nil {
		return nil, err
	}
	rawBytes, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return &apiextensionsv1.JSON{Raw: rawBytes}, err
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

	majorVersion := osVer.Segments()[0]
	roleMap := map[int]map[string]string{
		1: {
			"cluster_manager": "master",
		},
		2: {
			"master": "cluster_manager",
			"warm":   "search",
		},
		3: {
			"master": "cluster_manager",
		},
	}

	if mappedRole, ok := roleMap[majorVersion][role]; ok {
		return mappedRole
	}

	return role
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
	// diff := []string{}
	var diff []string

	for _, leftSliceString := range leftSlice {
		if !ContainsString(rightSlice, leftSliceString) {
			diff = append(diff, leftSliceString)
		}
	}
	return diff
}

// Count the number of pods running and ready and not terminating for a given nodePool
func CountRunningPodsForNodePool(k8sClient k8s.K8sClient, cr *opsterv1.OpenSearchCluster, nodePool *opsterv1.NodePool) (int, error) {
	// Constrict selector from labels
	clusterReq, err := labels.NewRequirement(ClusterLabel, selection.Equals, []string{cr.Name})
	if err != nil {
		return 0, err
	}
	componentReq, err := labels.NewRequirement(NodePoolLabel, selection.Equals, []string{nodePool.Component})
	if err != nil {
		return 0, err
	}
	selector := labels.NewSelector()
	selector = selector.Add(*clusterReq, *componentReq)
	// List pods matching selector
	list, err := k8sClient.ListPods(&client.ListOptions{Namespace: cr.Namespace, LabelSelector: selector})
	if err != nil {
		return 0, err
	}
	// Count pods that are ready
	numReadyPods := 0
	for _, pod := range list.Items {
		// If DeletionTimestamp is set the pod is terminating
		podReady := pod.DeletionTimestamp == nil
		// Count the pod as not ready if one of its containers is not running or not ready
		for _, container := range pod.Status.ContainerStatuses {
			if !container.Ready || container.State.Running == nil {
				podReady = false
			}
		}
		if podReady {
			numReadyPods += 1
		}
	}
	return numReadyPods, nil
}

// ReadyReplicasForNodePool returns the number of ready replicas derived from the actual running pods.
func ReadyReplicasForNodePool(k8sClient k8s.K8sClient, cr *opsterv1.OpenSearchCluster, nodePool *opsterv1.NodePool) (int32, error) {
	numReadyPods, err := CountRunningPodsForNodePool(k8sClient, cr, nodePool)
	if err != nil {
		return 0, err
	}
	return int32(numReadyPods), nil
}

// Count the number of PVCs created for the given NodePool
func CountPVCsForNodePool(k8sClient k8s.K8sClient, cr *opsterv1.OpenSearchCluster, nodePool *opsterv1.NodePool) (int, error) {
	clusterReq, err := labels.NewRequirement(ClusterLabel, selection.Equals, []string{cr.Name})
	if err != nil {
		return 0, err
	}
	componentReq, err := labels.NewRequirement(NodePoolLabel, selection.Equals, []string{nodePool.Component})
	if err != nil {
		return 0, err
	}
	selector := labels.NewSelector()
	selector = selector.Add(*clusterReq, *componentReq)
	list, err := k8sClient.ListPVCs(&client.ListOptions{Namespace: cr.Namespace, LabelSelector: selector})
	if err != nil {
		return 0, err
	}
	return len(list.Items), nil
}

// Delete a STS with cascade=orphan and wait until it is actually deleted from the kubernetes API
func WaitForSTSDelete(ctx context.Context, k8sClient k8s.K8sClient, obj *appsv1.StatefulSet) error {
	cond := func(ctx context.Context) (bool, error) {
		_, err := k8sClient.GetStatefulSet(obj.Name, obj.Namespace)
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}
	if err := k8sClient.DeleteStatefulSet(obj, true); err != nil {
		return err
	}
	return wait.PollUntilContextTimeout(ctx, time.Second*updateStepTime, time.Second*stsUpdateWaitTime, true, cond)
}

// Wait for max 30s until a STS has at least the given number of replicas
func WaitForSTSReplicas(ctx context.Context, k8sClient k8s.K8sClient, obj *appsv1.StatefulSet, replicas int32) error {
	cond := func(ctx context.Context) (bool, error) {
		existing, err := k8sClient.GetStatefulSet(obj.Name, obj.Namespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		if existing.Status.Replicas >= replicas {
			return true, nil
		}
		return false, nil
	}
	return wait.PollUntilContextTimeout(ctx, time.Second*updateStepTime, time.Second*stsUpdateWaitTime, true, cond)
}

func WaitForSTSStatus(ctx context.Context, k8sClient k8s.K8sClient, obj *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	var result appsv1.StatefulSet
	cond := func(ctx context.Context) (bool, error) {
		existing, err := k8sClient.GetStatefulSet(obj.Name, obj.Namespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		result = existing
		if existing.Status.CurrentRevision != "" {
			return true, nil
		}
		return false, nil
	}
	err := wait.PollUntilContextTimeout(ctx, time.Second*updateStepTime, time.Second*stsUpdateWaitTime, true, cond)
	return &result, err
}

// GetSTSForNodePool returns the corresponding sts for a given nodePool and cluster name
func GetSTSForNodePool(k8sClient k8s.K8sClient, nodePool opsterv1.NodePool, clusterName, clusterNamespace string) (*appsv1.StatefulSet, error) {
	stsName := clusterName + "-" + nodePool.Component
	existing, err := k8sClient.GetStatefulSet(stsName, clusterNamespace)
	return &existing, err
}

// DeleteSTSForNodePool deletes the sts for the corresponding nodePool
func DeleteSTSForNodePool(ctx context.Context, k8sClient k8s.K8sClient, nodePool opsterv1.NodePool, clusterName, clusterNamespace string) error {
	sts, err := GetSTSForNodePool(k8sClient, nodePool, clusterName, clusterNamespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if err := k8sClient.DeleteStatefulSet(sts, false); err != nil {
		return err
	}

	// Wait for the STS to actually be deleted using context-aware polling
	return wait.PollUntilContextTimeout(ctx, time.Second*updateStepTime, time.Second*stsUpdateWaitTime, true,
		func(ctx context.Context) (bool, error) {
			_, err := k8sClient.GetStatefulSet(sts.Name, sts.Namespace)
			if k8serrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		},
	)
}

// DeleteSecurityUpdateJob deletes the securityconfig update job
func DeleteSecurityUpdateJob(k8sClient k8s.K8sClient, clusterName, clusterNamespace string) error {
	jobName := clusterName + "-securityconfig-update"
	job, err := k8sClient.GetJob(jobName, clusterNamespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return k8sClient.DeleteJob(&job)
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

func ComposePDB(cr *opsterv1.OpenSearchCluster, nodepool *opsterv1.NodePool) policyv1.PodDisruptionBudget {
	matchLabels := map[string]string{
		ClusterLabel:  cr.Name,
		NodePoolLabel: nodepool.Component,
	}
	newpdb := policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-" + nodepool.Component + "-pdb",
			Namespace: cr.Namespace,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable:   nodepool.Pdb.MinAvailable,
			MaxUnavailable: nodepool.Pdb.MaxUnavailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
		},
	}
	return newpdb
}

func AppendJvmHeapSizeSettings(jvm string, heapSizeSettings string) string {
	if strings.Contains(jvm, "Xms") || strings.Contains(jvm, "Xmx") {
		return jvm
	}
	if jvm == "" {
		return heapSizeSettings
	}
	return fmt.Sprintf("%s %s", jvm, heapSizeSettings)
}

func CalculateJvmHeapSizeSettings(memoryRequest *resource.Quantity) string {
	var memoryRequestMb int64 = 512
	if memoryRequest != nil && !memoryRequest.IsZero() {
		memoryRequestMb = ((memoryRequest.Value() / 2.0) / 1024.0) / 1024.0
	}
	// Set Java Heap size to half of the node pool memory request for both Xms and Xmx
	return fmt.Sprintf("-Xms%dM -Xmx%dM", memoryRequestMb, memoryRequestMb)
}

func IsUpgradeInProgress(status opsterv1.ClusterStatus) bool {
	componentStatus := opsterv1.ComponentStatus{
		Component: "Upgrader",
	}
	foundStatus := FindAllPartial(status.ComponentsStatus, componentStatus, GetByComponent)
	inProgress := false

	// check all statuses if any of the nodepools are still in progress or pending
	for i := 0; i < len(foundStatus); i++ {
		if foundStatus[i].Status != "Upgraded" && foundStatus[i].Status != "Finished" {
			inProgress = true
		}
	}

	return inProgress
}

func ReplicaHostName(currentSts appsv1.StatefulSet, repNum int32) string {
	return fmt.Sprintf("%s-%d", currentSts.Name, repNum)
}

func WorkingPodForRollingRestart(k8sClient k8s.K8sClient, sts *appsv1.StatefulSet) (string, error) {
	// If there are potentially mixed revisions we need to check each pod
	podWithOlderRevision, err := GetPodWithOlderRevision(k8sClient, sts)
	if err != nil {
		return "", err
	}
	if podWithOlderRevision != nil {
		return podWithOlderRevision.Name, nil
	}
	return "", errors.New("unable to calculate the working pod for rolling restart")
}

// DeleteStuckPodWithOlderRevision deletes the crashed pod only if there is any update in StatefulSet.
func DeleteStuckPodWithOlderRevision(k8sClient k8s.K8sClient, sts *appsv1.StatefulSet) error {
	podWithOlderRevision, err := GetPodWithOlderRevision(k8sClient, sts)
	if err != nil {
		return err
	}
	if podWithOlderRevision != nil {
		for _, container := range podWithOlderRevision.Status.ContainerStatuses {
			// If any container is getting crashed, restart it by deleting the pod so that new update in sts can take place.
			if !container.Ready && container.State.Waiting != nil && container.State.Waiting.Reason == "CrashLoopBackOff" {
				return k8sClient.DeletePod(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podWithOlderRevision.Name,
						Namespace: sts.Namespace,
					},
				})
			}
		}
	}
	return nil
}

// GetPodWithOlderRevision fetches the pod that is not having the updated revision.
func GetPodWithOlderRevision(k8sClient k8s.K8sClient, sts *appsv1.StatefulSet) (*corev1.Pod, error) {
	for i := int32(0); i < lo.FromPtrOr(sts.Spec.Replicas, 1); i++ {
		podName := ReplicaHostName(*sts, i)
		pod, err := k8sClient.GetPod(podName, sts.Namespace)
		if err != nil {
			return nil, err
		}
		podRevision, ok := pod.Labels[stsRevisionLabel]
		if !ok {
			return nil, fmt.Errorf("pod %s has no revision label", podName)
		}
		if podRevision != sts.Status.UpdateRevision {
			return &pod, nil
		}
	}
	return nil, nil
}

func GetDashboardsDeployment(k8sClient k8s.K8sClient, clusterName, clusterNamespace string) (*appsv1.Deployment, error) {
	deploy, err := k8sClient.GetDeployment(clusterName+"-dashboards", clusterNamespace)
	return &deploy, err
}

// DeleteDashboardsDeployment deletes the OSD deployment along with all its pods
func DeleteDashboardsDeployment(ctx context.Context, k8sClient k8s.K8sClient, clusterName, clusterNamespace string) error {
	deploy, err := GetDashboardsDeployment(k8sClient, clusterName, clusterNamespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if err := k8sClient.DeleteDeployment(deploy, false); err != nil {
		return err
	}

	// Wait for Dashboards deploy to delete using context-aware polling
	return wait.PollUntilContextTimeout(ctx, time.Second*updateStepTime, time.Second*stsUpdateWaitTime, true,
		func(ctx context.Context) (bool, error) {
			_, err := k8sClient.GetDeployment(deploy.Name, clusterNamespace)
			if k8serrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		},
	)
}

func SafeClose(c io.Closer) {
	if err := c.Close(); err != nil {
		log.Println("SafeClose error:", err)
	}
}

// ResolveUidGid resolves the UID and GID using security context hierarchy
// Priority: securityContext.runAsUser/Group > podSecurityContext.runAsUser/Group > defaults (1000:1000)
func ResolveUidGid(cr *opsterv1.OpenSearchCluster) (uid, gid int64) {
	uid = DefaultUID
	gid = DefaultGID

	if cr.Spec.General.SecurityContext != nil && cr.Spec.General.SecurityContext.RunAsUser != nil {
		uid = *cr.Spec.General.SecurityContext.RunAsUser
	} else if cr.Spec.General.PodSecurityContext != nil && cr.Spec.General.PodSecurityContext.RunAsUser != nil {
		uid = *cr.Spec.General.PodSecurityContext.RunAsUser
	}

	if cr.Spec.General.SecurityContext != nil && cr.Spec.General.SecurityContext.RunAsGroup != nil {
		gid = *cr.Spec.General.SecurityContext.RunAsGroup
	} else if cr.Spec.General.PodSecurityContext != nil && cr.Spec.General.PodSecurityContext.RunAsGroup != nil {
		gid = *cr.Spec.General.PodSecurityContext.RunAsGroup
	}

	return uid, gid
}

// GetChownCommand creates a chown command with the given UID, GID, and path
func GetChownCommand(uid, gid int64, path string) string {
	return fmt.Sprintf("chown -R %d:%d %s", uid, gid, path)
}

// GenComponentTemplateName generates the component template name from the resource
func GenComponentTemplateName(template *opsterv1.OpensearchComponentTemplate) string {
	if template.Spec.Name != "" {
		return template.Spec.Name
	}
	return template.Name
}

// GenIndexTemplateName generates the index template name from the resource
func GenIndexTemplateName(template *opsterv1.OpensearchIndexTemplate) string {
	if template.Spec.Name != "" {
		return template.Spec.Name
	}
	return template.Name
}

func DiscoverRandomAdminSecret(k8sClient k8s.K8sClient, cr *opsterv1.OpenSearchCluster) (*corev1.Secret, error) {
	if cr.Spec.Security == nil || cr.Spec.Security.Config == nil {
		return nil, fmt.Errorf("security config is not defined")
	}
	if cr.Spec.Security.Config.AdminCredentialsSecret.Name != "" {
		return nil, fmt.Errorf("admin credentials secret managed by user")
	}
	secret, err := k8sClient.GetSecret(GeneratedAdminCredentialsSecretName(cr), cr.Namespace)
	return &secret, err
}

func DiscoverRandomContextSecret(k8sClient k8s.K8sClient, cr *opsterv1.OpenSearchCluster) (*corev1.Secret, error) {
	secret, err := k8sClient.GetSecret(GeneratedSecurityConfigSecretName(cr), cr.Namespace)
	return &secret, err
}

// EnsureDashboardsCredentialsSecret ensures a credentials secret exists for Dashboards.
// It generates a separate password for Dashboards (not the admin password).
func EnsureDashboardsCredentialsSecret(k8sClient k8s.K8sClient, cr *opsterv1.OpenSearchCluster) (*corev1.Secret, bool, error) {
	// Check if user provided OpensearchCredentialsSecret via Dashboards config
	if cr.Spec.Dashboards.OpensearchCredentialsSecret.Name != "" {
		secret, err := k8sClient.GetSecret(cr.Spec.Dashboards.OpensearchCredentialsSecret.Name, cr.Namespace)
		return &secret, false, err
	}

	// Always generate/administer the Dashboards credentials secret
	generatedName := GeneratedDashboardsCredentialsSecretName(cr)
	secret, err := k8sClient.GetSecret(generatedName, cr.Namespace)
	if err == nil {
		return &secret, true, nil
	}
	if !k8serrors.IsNotFound(err) {
		return nil, true, err
	}

	randomPassword := rand.Text()
	// NOTE(joseb): we cannot set random password when security plugin is disabled.
	if !IsSecurityPluginEnabled(cr) {
		randomPassword = "kibanaserver"
	}

	dashboardsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generatedName,
			Namespace: cr.Namespace,
		},
		StringData: map[string]string{
			"username": "kibanaserver",
			"password": randomPassword,
		},
	}
	if _, err := k8sClient.CreateSecret(dashboardsSecret); err != nil {
		return nil, true, err
	}

	createdSecret, err := k8sClient.GetSecret(generatedName, cr.Namespace)
	if err != nil {
		return nil, true, err
	}
	return &createdSecret, true, nil
}
