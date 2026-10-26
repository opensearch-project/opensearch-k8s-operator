package reconcilers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	"gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// appliedStateAnnotation records on an update job the securityconfig state the cluster is in once the job succeeded.
	appliedStateAnnotation = "securityconfig/applied-state"

	internalUsersFile = "internal_users.yml"
)

// securityConfigPlan is what has to be re-applied to bring an initialized cluster from the last
// applied securityconfig to the current one.
type securityConfigPlan struct {
	// files are securityconfig files supplied by the user whose content changed. They are re-applied
	// with securityadmin, which replaces the whole document of each file's type.
	files []string
	// skippedDefaults are bundled default files whose content changed, e.g. after an operator upgrade.
	// Bundled defaults are only applied when the cluster is set up, so that they never overwrite
	// security objects created later through the REST API or Dashboards.
	skippedDefaults []string
	// patchManagedUsers is set when the admin or Dashboards user password hash changed and
	// internal_users.yml is not re-applied anyway. The Dashboards user is kibanaserver unless
	// the credentials secret names another user.
	patchManagedUsers bool
}

// securityConfigState computes the state recorded for a securityconfig: a checksum per file as
// supplied (or bundled), and one for the operator-managed password hashes.
// dashboardsUsername is the internal user Dashboards authenticates as.
func securityConfigState(source *helpers.SecurityConfigSource, generated *corev1.Secret, dashboardsUsername string) (opensearchv1.SecurityConfigStatus, error) {
	state := opensearchv1.SecurityConfigStatus{AppliedChecksums: map[string]string{}}
	for name, content := range source.Data {
		if _, ok := ymlToFileType[name]; ok && len(content) > 0 {
			state.AppliedChecksums[name] = contentChecksum(content)
		}
	}

	adminHash, dashboardsHash, err := managedPasswordHashes(generated.Data[internalUsersFile], dashboardsUsername)
	if err != nil {
		return state, err
	}
	state.ManagedUsersChecksum = managedUsersChecksum(adminHash, dashboardsUsername, dashboardsHash)
	return state, nil
}

// managedPasswordHashes reads the admin and Dashboards user hashes from a generated internal_users.yml.
func managedPasswordHashes(internalUsers []byte, dashboardsUsername string) (string, string, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal(internalUsers, &data); err != nil {
		return "", "", fmt.Errorf("unable to parse generated %s: %w", internalUsersFile, err)
	}
	return internalUserHash(data, "admin"), internalUserHash(data, dashboardsUsername), nil
}

// internalUserHash returns the hash of username. yaml.v2 decodes nested mappings as map[interface{}]interface{}.
func internalUserHash(data map[string]interface{}, username string) string {
	if username == "" {
		return ""
	}
	user, ok := data[username].(map[interface{}]interface{})
	if !ok {
		return ""
	}
	hash, _ := user["hash"].(string)
	return hash
}

// managedUsersChecksum covers the admin hash and, when the Dashboards user is someone else, that user's hash.
// A kibanaserver entry left beside a custom Dashboards user is not operator-managed and is ignored.
func managedUsersChecksum(adminHash, dashboardsUsername, dashboardsHash string) string {
	hashes := adminHash
	if dashboardsUsername != "admin" && dashboardsHash != "" {
		hashes += "\n" + dashboardsHash
	}
	return contentChecksum([]byte(hashes))
}

func contentChecksum(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// securityConfigStatesEqual compares the file and password hash checksums of two states.
func securityConfigStatesEqual(a, b opensearchv1.SecurityConfigStatus) bool {
	if a.ManagedUsersChecksum != b.ManagedUsersChecksum || len(a.AppliedChecksums) != len(b.AppliedChecksums) {
		return false
	}
	for name, sum := range a.AppliedChecksums {
		if b.AppliedChecksums[name] != sum {
			return false
		}
	}
	return true
}

// changedSecurityConfigFiles returns the files of to whose content differs from from, sorted.
func changedSecurityConfigFiles(from, to opensearchv1.SecurityConfigStatus) []string {
	var changed []string
	for name, sum := range to.AppliedChecksums {
		if from.AppliedChecksums[name] != sum {
			changed = append(changed, name)
		}
	}
	sort.Strings(changed)
	return changed
}

// planSecurityConfigUpdate compares the last applied state with the current one. dirty lists files
// that an unfinished update job may already have applied; they are re-applied from the current content.
func planSecurityConfigUpdate(applied, current opensearchv1.SecurityConfigStatus, userKeys map[string]bool, dirty []string) securityConfigPlan {
	changed := changedSecurityConfigFiles(applied, current)
	for _, name := range dirty {
		if _, ok := current.AppliedChecksums[name]; ok && !slices.Contains(changed, name) {
			changed = append(changed, name)
		}
	}
	sort.Strings(changed)

	var plan securityConfigPlan
	for _, name := range changed {
		if userKeys[name] {
			plan.files = append(plan.files, name)
		} else {
			plan.skippedDefaults = append(plan.skippedDefaults, name)
		}
	}
	plan.patchManagedUsers = applied.ManagedUsersChecksum != current.ManagedUsersChecksum &&
		!slices.Contains(plan.files, internalUsersFile)
	return plan
}

// jobAppliedState returns the state recorded on an update job, if any.
func jobAppliedState(job batchv1.Job) (opensearchv1.SecurityConfigStatus, bool) {
	var state opensearchv1.SecurityConfigStatus
	value, ok := job.Annotations[appliedStateAnnotation]
	if !ok || json.Unmarshal([]byte(value), &state) != nil {
		return state, false
	}
	return state, true
}

func setJobAppliedState(job *batchv1.Job, state opensearchv1.SecurityConfigStatus) error {
	value, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if job.Annotations == nil {
		job.Annotations = map[string]string{}
	}
	job.Annotations[appliedStateAnnotation] = string(value)
	return nil
}

// filterSecurityConfigSecret returns a copy of secret that only contains the given files.
func filterSecurityConfigSecret(secret *corev1.Secret, files []string) *corev1.Secret {
	filtered := secret.DeepCopy()
	filtered.Data = make(map[string][]byte, len(files))
	for _, name := range files {
		filtered.Data[name] = secret.Data[name]
	}
	return filtered
}

// recordAppliedSecurityConfig stores state as the last applied securityconfig.
func (r *SecurityconfigReconciler) recordAppliedSecurityConfig(state opensearchv1.SecurityConfigStatus) error {
	err := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
		instance.Status.SecurityConfig = state.DeepCopy()
	})
	if err != nil {
		return err
	}
	r.instance.Status.SecurityConfig = state.DeepCopy()
	return nil
}

// recordSucceededJob records the state a successful update job applied, and returns the last
// applied state, or nil when it is not known.
func (r *SecurityconfigReconciler) recordSucceededJob(
	job batchv1.Job,
	jobExists bool,
	checksumval string,
	current opensearchv1.SecurityConfigStatus,
) (*opensearchv1.SecurityConfigStatus, error) {
	applied := r.instance.Status.SecurityConfig
	if !jobExists || job.Status.Succeeded == 0 {
		return applied, nil
	}
	jobChecksum := job.Annotations[checksumAnnotation]
	if applied != nil && applied.UpdateJobChecksum == jobChecksum {
		return applied, nil
	}

	target, ok := jobAppliedState(job)
	if !ok {
		// Jobs created by earlier operator versions do not record the state they applied. Such a
		// job applied the current securityconfig if its checksum matches.
		if applied != nil || jobChecksum != checksumval {
			return applied, nil
		}
		target = current
	}
	target.UpdateJobChecksum = jobChecksum
	if err := r.recordAppliedSecurityConfig(target); err != nil {
		return nil, err
	}
	return r.instance.Status.SecurityConfig, nil
}

// patchManagedUserHashes updates the admin and Dashboards user password hashes through the security
// REST API. Unlike re-applying internal_users.yml with securityadmin, this keeps all other internal
// users, e.g. those created through the REST API or Dashboards.
func (r *SecurityconfigReconciler) patchManagedUserHashes(generated *corev1.Secret, adminCertName, dashboardsUsername string) error {
	adminHash, dashboardsHash, err := managedPasswordHashes(generated.Data[internalUsersFile], dashboardsUsername)
	if err != nil {
		return err
	}
	hashes := map[string]string{"admin": adminHash}
	names := []string{"admin"}
	if dashboardsUsername != "admin" && dashboardsHash != "" {
		hashes[dashboardsUsername] = dashboardsHash
		names = append(names, dashboardsUsername)
	}

	osClient, err := util.CreateAdminCertClientForCluster(
		r.client,
		r.instance,
		adminCertName,
		r.determineAdminCASecret(adminCertName),
		r.options.osClientTransport,
	)
	if err != nil {
		return err
	}

	for _, name := range names {
		hash := hashes[name]
		if hash == "" {
			continue
		}
		body, err := json.Marshal([]map[string]string{{"op": "add", "path": "/hash", "value": hash}})
		if err != nil {
			return err
		}
		res, err := osClient.PatchSecurityResource(r.ctx, "internalusers", name, bytes.NewReader(body))
		if err != nil {
			return err
		}
		detail, readErr := readLimitedBody(res.Body)
		if res.StatusCode != http.StatusOK {
			if readErr != nil {
				return fmt.Errorf("updating the password hash of %s returned HTTP %d: %w", name, res.StatusCode, readErr)
			}
			if detail == "" {
				return fmt.Errorf("updating the password hash of %s returned HTTP %d", name, res.StatusCode)
			}
			return fmt.Errorf("updating the password hash of %s returned HTTP %d: %s", name, res.StatusCode, detail)
		}
		if readErr != nil {
			return fmt.Errorf("reading the password hash update response for %s: %w", name, readErr)
		}
	}
	return nil
}

// readLimitedBody reads and closes body, keeping at most 4KiB for error messages.
func readLimitedBody(body io.ReadCloser) (string, error) {
	if body == nil {
		return "", nil
	}
	payload, err := io.ReadAll(io.LimitReader(body, 4096))
	closeErr := body.Close()
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}
	if closeErr != nil {
		return "", fmt.Errorf("closing response body: %w", closeErr)
	}
	return strings.TrimSpace(string(payload)), nil
}
