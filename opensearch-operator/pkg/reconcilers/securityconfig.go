package reconcilers

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconciler"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	securityConfigReconcilerName = "securityconfig"
	securityConfigComponentName  = "Securityconfig"

	checksumAnnotation = "securityconfig/checksum"

	securityConfigStatusReady   = "Ready"
	securityConfigStatusRunning = "Running"
	securityConfigStatusFailed  = "Failed"

	securityConfigRetryConditionPrefix     = "retry:"
	securityConfigLastRetryConditionPrefix = "lastRetry:"

	securityConfigInitialRetryDelay = 30 * time.Second
	securityConfigMaxRetryDelay     = 15 * time.Minute

	securityConfigConnectWaitAttempts = 60

	adminCert = "/certs/tls.crt"
	adminKey  = "/certs/tls.key"
	caCert    = "/certs/ca.crt"

	SecurityAdminBaseCmdTmpl = `ADMIN=%s/plugins/opensearch-security/tools/securityadmin.sh;
chmod +x $ADMIN;
wait_count=0;
until curl -k --silent https://%s:%v;
do
  if (( wait_count++ >= %d )); then
    echo "Failed to connect to cluster after %d attempts";
    exit 1;
  fi;
  echo 'Waiting to connect to the cluster'; sleep 20;
done;`

	BackupSecurityConfigCmdTmpl = `
BACKUP_DIR=/tmp/security-backup;
mkdir -p $BACKUP_DIR;
echo "Backing up current security configuration...";
count=0;
until $ADMIN -backup $BACKUP_DIR -cacert %s -cert %s -key %s -icl -nhnv -h %s -p %v; do
  if (( count++ >= 20 )); then
    echo "ERROR: Failed to backup security configuration after 20 attempts";
    exit 1;
  fi;
  echo "Backup attempt failed, retrying...";
  sleep 20;
done;
echo "Backup completed successfully";`

	MergeInternalUsersCmdTmpl = `
# Use yq from tools volume
YQ_CMD=/tools/yq;

# Verify yq is available
if [ ! -x "$YQ_CMD" ]; then
  echo "ERROR: yq not found at $YQ_CMD";
  exit 1;
fi;

# Merge internal_users.yml (custom overrides existing)
BACKUP_USERS=/tmp/security-backup/internal_users.yml;
CUSTOM_USERS=%s/internal_users.yml;
MERGED_USERS=/tmp/merged_internal_users.yml;

if [ -f "$BACKUP_USERS" ] && [ -f "$CUSTOM_USERS" ]; then
  echo "Merging internal_users.yml (custom config takes precedence)...";
  $YQ_CMD eval-all 'select(fileIndex == 0) * select(fileIndex == 1)' \
    $BACKUP_USERS $CUSTOM_USERS > $MERGED_USERS || {
      echo "ERROR: Failed to merge internal_users.yml";
      exit 1;
    };
  echo "Merge completed successfully - merged file at $MERGED_USERS";
elif [ ! -f "$CUSTOM_USERS" ]; then
  echo "No custom internal_users.yml found, using backup";
  cp $BACKUP_USERS $MERGED_USERS || {
    echo "ERROR: Failed to copy backup internal_users.yml";
    exit 1;
  };
else
  echo "No backup found, using custom internal_users.yml as-is";
  cp $CUSTOM_USERS $MERGED_USERS || {
    echo "ERROR: Failed to copy custom internal_users.yml";
    exit 1;
  };
fi;`

	ApplyAllYmlCmdTmpl = `count=0;
until $ADMIN -cacert %s -cert %s -key %s -cd %s -icl -nhnv -h %s -p %v; do
  if (( count++ >= 20 )); then
    echo "Failed to apply securityconfig after 20 attempts";
    exit 1;
  fi;
  sleep 20;
done;`

	ApplySingleYmlCmdTmpl = `count=0;
until $ADMIN -cacert %s -cert %s -key %s -f %s -t %s -icl -nhnv -h %s -p %v; do
  if (( count++ >= 20 )); then
    echo "Failed to apply securityconfig after 20 attempts";
    exit 1;
  fi;
  sleep 20;
done;`
)

var ymlToFileType = map[string]string{
	"internal_users.yml": "internalusers",
	"roles.yml":          "roles",
	"roles_mapping.yml":  "rolesmapping",
	"action_groups.yml":  "actiongroups",
	"tenants.yml":        "tenants",
	"nodes_dn.yml":       "nodesdn",
	"whitelist.yml":      "allowlist",
	"audit.yml":          "audit",
	"allowlist.yml":      "allowlist",
	"config.yml":         "config",
}

type SecurityconfigReconciler struct {
	client            k8s.K8sClient
	recorder          record.EventRecorder
	reconcilerContext *ReconcilerContext
	instance          *opensearchv1.OpenSearchCluster
	logger            logr.Logger
}

func NewSecurityconfigReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	reconcilerContext *ReconcilerContext,
	instance *opensearchv1.OpenSearchCluster,
	opts ...reconciler.ResourceReconcilerOption,
) *SecurityconfigReconciler {
	return &SecurityconfigReconciler{
		client:            k8s.NewK8sClient(client, ctx, append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", securityConfigReconcilerName)))...),
		reconcilerContext: reconcilerContext,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx),
	}
}

func (r *SecurityconfigReconciler) Name() string { return securityConfigReconcilerName }

func (r *SecurityconfigReconciler) Reconcile() (ctrl.Result, error) {
	if !helpers.IsSecurityPluginEnabled(r.instance) {
		r.logger.Info("Security plugin is disabled, skipping securityconfig reconciliation")
		return ctrl.Result{}, nil
	}
	annotations := map[string]string{"cluster-name": r.instance.GetName()}

	var configSecretName string
	var checksumval string
	var cmdArg string

	adminCertName := r.determineAdminSecret()
	namespace := r.instance.Namespace
	clusterName := r.instance.Name
	jobName := clusterName + "-securityconfig-update"

	configSecretName = helpers.GeneratedSecurityConfigSecretName(r.instance)

	// TODO(joseb): Check if admin certificate is provided or generated in webhook
	if adminCertName == "" {
		err := errors.New("admin certificate neither provided nor generation is enabled")
		r.logger.Error(err, "Skipping securityconfig reconciliation")
		return ctrl.Result{}, err
	}

	adminCredentialsSecret, managedByOperator, err := helpers.EnsureAdminCredentialsSecret(r.client, r.instance)
	if err != nil {
		r.logger.Error(err, "Unable to ensure admin credentials secret")
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 30}, err
	}

	if managedByOperator != r.instance.Status.AdminSecretCreated {
		updateErr := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
			instance.Status.AdminSecretCreated = managedByOperator
		})
		if updateErr != nil {
			r.logger.Error(updateErr, "Unable to update admin secret creation status")
			return ctrl.Result{}, updateErr
		}
	}

	generatedConfigSecret, err := helpers.BuildGeneratedSecurityConfigSecret(r.client, r.instance, adminCredentialsSecret)
	if err != nil {
		r.logger.Error(err, "Unable to build generated security config secret")
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 30}, err
	}

	if err := ctrl.SetControllerReference(r.instance, generatedConfigSecret, r.client.Scheme()); err != nil {
		return ctrl.Result{}, err
	}
	if _, err := r.client.ReconcileResource(generatedConfigSecret, reconciler.StatePresent); err != nil {
		return ctrl.Result{}, err
	}

	if !r.instance.Status.ContextSecretCreated {
		updateErr := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opensearchv1.OpenSearchCluster) {
			instance.Status.ContextSecretCreated = true
		})
		if updateErr != nil {
			r.logger.Error(updateErr, "Unable to update security context secret creation status")
			return ctrl.Result{}, updateErr
		}
	}

	configSecret, err := r.client.GetSecret(configSecretName, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Info(fmt.Sprintf("Waiting for generated secret '%s' that contains the securityconfig to be created", configSecretName))
			r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Security", "Waiting for generated secret '%s' that contains the securityconfig", configSecretName)
			return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 10}, nil
		}
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 10}, err
	}

	var checksumerr error
	checksumval, checksumerr = checksum(configSecret.Data)
	if checksumerr != nil {
		return ctrl.Result{}, checksumerr
	}
	if err := r.securityconfigSubpaths(r.instance, &configSecret); err != nil {
		return ctrl.Result{}, err
	}
	cmdArg = BuildCmdArg(r.instance, &configSecret, r.logger)

	job, err := r.client.GetJob(jobName, namespace)
	resetRetryCount := false
	if err == nil {
		value, exists := job.Annotations[checksumAnnotation]
		if exists && value == checksumval {
			result, done, handleErr := r.handleExistingSecurityConfigJob(job, annotations)
			if handleErr != nil {
				return ctrl.Result{}, handleErr
			}
			if done {
				return result, nil
			}
			// Failed job past backoff window: delete and recreate below.
		} else {
			resetRetryCount = true
		}
		// Delete old job
		r.logger.Info("Deleting old update job")
		err := r.client.DeleteJob(&job)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	if resetRetryCount {
		if err := r.updateSecurityConfigComponentStatus(securityConfigStatusRunning, "", r.securityConfigRetryConditions(0, time.Time{})); err != nil {
			r.logger.Error(err, "Unable to reset securityconfig retry status")
			return ctrl.Result{}, err
		}
	} else if err := r.updateSecurityConfigComponentStatus(securityConfigStatusRunning, "", r.currentSecurityConfigRetryConditions()); err != nil {
		r.logger.Error(err, "Unable to update securityconfig status")
		return ctrl.Result{}, err
	}

	// If the cluster has not yet initialized or
	// securityconfig secret was not passed, build the command to apply all yml files
	if !r.instance.Status.Initialized || len(cmdArg) == 0 {
		clusterHostName := BuildClusterSvcHostName(r.instance)
		opensearchHome := r.instance.Spec.General.GetOpenSearchHome()
		httpPort, securityConfigPort, securityconfigPath := helpers.VersionCheck(r.instance)
		cmdArg = fmt.Sprintf(SecurityAdminBaseCmdTmpl, opensearchHome, clusterHostName, httpPort, securityConfigConnectWaitAttempts, securityConfigConnectWaitAttempts) +
			fmt.Sprintf(ApplyAllYmlCmdTmpl, caCert, adminCert, adminKey, securityconfigPath, clusterHostName, securityConfigPort)
	}

	r.logger.Info("Starting securityconfig update job")
	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Security", "Starting securityconfig update job")

	job = builders.NewSecurityconfigUpdateJob(
		r.instance,
		jobName,
		namespace,
		checksumval,
		adminCertName,
		r.determineAdminCASecret(adminCertName),
		cmdArg,
		r.reconcilerContext.Volumes,
		r.reconcilerContext.VolumeMounts,
	)

	if err := ctrl.SetControllerReference(r.instance, &job, r.client.Scheme()); err != nil {
		return ctrl.Result{}, err
	}

	_, err = r.client.CreateJob(&job)
	return ctrl.Result{}, err
}

// BuildCmdArg builds the command for the securityconfig-update job for each individual ymls present in the
// securityconfig secret. yml files which are not present in the secret are not applied/updated
func BuildCmdArg(instance *opensearchv1.OpenSearchCluster, secret *corev1.Secret, log logr.Logger) string {
	clusterHostName := BuildClusterSvcHostName(instance)
	opensearchHome := instance.Spec.General.GetOpenSearchHome()
	httpPort, securityConfigPort, securityconfigPath := helpers.VersionCheck(instance)

	arg := fmt.Sprintf(SecurityAdminBaseCmdTmpl, opensearchHome, clusterHostName, httpPort, securityConfigConnectWaitAttempts, securityConfigConnectWaitAttempts)

	// Check if we should merge internal users (default behavior)
	shouldMerge := helpers.ShouldMergeSecurityConfig(instance)

	if shouldMerge {
		log.Info("Security config merge mode enabled - will preserve existing internal users")
		// Add backup command
		arg += fmt.Sprintf(BackupSecurityConfigCmdTmpl, caCert, adminCert, adminKey, clusterHostName, securityConfigPort)

		// Add merge command
		arg += fmt.Sprintf(MergeInternalUsersCmdTmpl, securityconfigPath)
	} else {
		log.Info("Security config overwrite mode enabled - will replace all internal users")
	}

	// Get the list of yml files and sort them
	// This will ensure commands are always generated in the same order
	// Needed for tests as well to compare actual and expected command
	keys := make([]string, 0, len(secret.Data))
	for k := range secret.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		var filePath string

		// If merge mode is enabled and this is internal_users.yml, use the merged file from temp
		if shouldMerge && k == "internal_users.yml" {
			filePath = "/tmp/merged_internal_users.yml"
		} else {
			filePath = fmt.Sprintf("%s/%s", securityconfigPath, k)
		}

		fileType, ok := ymlToFileType[k]
		if !ok {
			// If the yml file is invalid, do not return the error
			// Just log it and build the commands for valid yml files
			log.Error(fmt.Errorf("invalid yml file %s in securityconfig secret", k), fmt.Sprintf("skipping %s", k))
			continue
		}
		// Necessary as kubectl apply for stringData doesn't completely remove the field from the secret
		// Even if the field was removed from the yaml file it was applied from
		// Instead it sets it to an empty value
		if string(secret.Data[k]) != "" {
			arg = arg + fmt.Sprintf(ApplySingleYmlCmdTmpl, caCert, adminCert, adminKey, filePath, fileType, clusterHostName, securityConfigPort)
		}
	}

	return arg
}

func checksum(data map[string][]byte) (string, error) {
	hash := sha1.New()
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		_, err := hash.Write([]byte(key))
		if err != nil {
			return "", err
		}
		value := data[key]
		_, err = hash.Write(value)
		if err != nil {
			return "", err
		}
	}
	return base64.StdEncoding.EncodeToString(hash.Sum(nil)), nil
}

func (r *SecurityconfigReconciler) determineAdminSecret() string {
	if r.instance.Spec.Security != nil {
		if r.instance.Spec.Security.Config != nil && r.instance.Spec.Security.Config.AdminSecret.Name != "" {
			return r.instance.Spec.Security.Config.AdminSecret.Name
		}
	}
	// Webhook validation ensures that if security plugin is enabled and no AdminSecret is provided,
	// then TLS Generate must be true. So we can safely return the default admin cert name.
	if helpers.IsSecurityPluginEnabled(r.instance) {
		return fmt.Sprintf("%s-admin-cert", r.instance.Name)
	}
	// Security plugin is not enabled, no admin cert needed
	return ""
}

func (r *SecurityconfigReconciler) determineAdminCASecret(adminSecretName string) string {
	caSecretName := helpers.TlsCASecretRef(r.instance).Name
	// If CA comes from the same secret, keep single-secret mounting behavior.
	if caSecretName == "" || caSecretName == adminSecretName {
		return ""
	}
	return caSecretName
}

func (r *SecurityconfigReconciler) DeleteResources() (ctrl.Result, error) {
	result := reconciler.CombinedResult{}
	return result.Result, result.Err
}

func (r *SecurityconfigReconciler) securityconfigSubpaths(instance *opensearchv1.OpenSearchCluster, secret *corev1.Secret) error {
	r.reconcilerContext.Volumes = append(r.reconcilerContext.Volumes, corev1.Volume{
		Name: "securityconfig",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secret.Name,
			},
		},
	})

	keys := make([]string, 0, len(secret.Data))
	for k := range secret.Data {
		keys = append(keys, k)
	}
	_, _, securityconfigPath := helpers.VersionCheck(instance)
	sort.Strings(keys)
	for _, k := range keys {
		r.reconcilerContext.VolumeMounts = append(r.reconcilerContext.VolumeMounts, corev1.VolumeMount{
			Name:      "securityconfig",
			MountPath: fmt.Sprintf("%s/%s", securityconfigPath, k),
			SubPath:   k,
			ReadOnly:  true,
		})
	}

	return nil
}

// BuildClusterSvcHostName builds the cluster host name as {svc-name}.{namespace}.svc.{dns-base}
func BuildClusterSvcHostName(instance *opensearchv1.OpenSearchCluster) string {
	return fmt.Sprintf("%s.svc.%s", builders.DnsOfService(instance), helpers.ClusterDnsBase())
}

func (r *SecurityconfigReconciler) handleExistingSecurityConfigJob(
	job batchv1.Job,
	annotations map[string]string,
) (ctrl.Result, bool, error) {
	if job.Status.Succeeded > 0 {
		if err := r.updateSecurityConfigComponentStatus(securityConfigStatusReady, "", nil); err != nil {
			return ctrl.Result{}, true, err
		}
		return ctrl.Result{}, true, nil
	}

	if job.Status.Active > 0 {
		if err := r.updateSecurityConfigComponentStatus(securityConfigStatusRunning, "", nil); err != nil {
			return ctrl.Result{}, true, err
		}
		return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, true, nil
	}

	if job.Status.Failed > 0 {
		retryCount := r.securityConfigRetryCount()
		delay := securityConfigRetryDelay(retryCount)
		lastRetry := r.securityConfigLastRetry()
		if !lastRetry.IsZero() && time.Since(lastRetry) < delay {
			remaining := delay - time.Since(lastRetry)
			if err := r.updateSecurityConfigComponentStatus(
				securityConfigStatusFailed,
				"securityconfig update job failed",
				r.securityConfigRetryConditions(retryCount, lastRetry),
			); err != nil {
				return ctrl.Result{}, true, err
			}
			return ctrl.Result{Requeue: true, RequeueAfter: remaining}, true, nil
		}

		retryCount++
		now := time.Now().UTC()
		r.recorder.AnnotatedEventf(
			r.instance,
			annotations,
			"Warning",
			"Security",
			"Securityconfig update job failed, retrying (attempt %d)",
			retryCount,
		)
		if err := r.updateSecurityConfigComponentStatus(
			securityConfigStatusFailed,
			"securityconfig update job failed",
			r.securityConfigRetryConditions(retryCount, now),
		); err != nil {
			return ctrl.Result{}, true, err
		}
		return ctrl.Result{}, false, nil
	}

	if err := r.updateSecurityConfigComponentStatus(securityConfigStatusRunning, "", nil); err != nil {
		return ctrl.Result{}, true, err
	}
	return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, true, nil
}

func (r *SecurityconfigReconciler) updateSecurityConfigComponentStatus(status, description string, conditions []string) error {
	return UpdateComponentStatus(r.client, r.instance, &opensearchv1.ComponentStatus{
		Component:   securityConfigComponentName,
		Status:      status,
		Description: description,
		Conditions:  conditions,
	})
}

func (r *SecurityconfigReconciler) securityConfigComponentStatus() (opensearchv1.ComponentStatus, bool) {
	for _, componentStatus := range r.instance.Status.ComponentsStatus {
		if componentStatus.Component == securityConfigComponentName {
			return componentStatus, true
		}
	}
	return opensearchv1.ComponentStatus{}, false
}

func (r *SecurityconfigReconciler) securityConfigRetryCount() int {
	componentStatus, found := r.securityConfigComponentStatus()
	if !found {
		return 0
	}
	for _, condition := range componentStatus.Conditions {
		if strings.HasPrefix(condition, securityConfigRetryConditionPrefix) {
			retryCount, err := strconv.Atoi(strings.TrimPrefix(condition, securityConfigRetryConditionPrefix))
			if err == nil {
				return retryCount
			}
		}
	}
	return 0
}

func (r *SecurityconfigReconciler) securityConfigLastRetry() time.Time {
	componentStatus, found := r.securityConfigComponentStatus()
	if !found {
		return time.Time{}
	}
	for _, condition := range componentStatus.Conditions {
		if strings.HasPrefix(condition, securityConfigLastRetryConditionPrefix) {
			lastRetry, err := time.Parse(time.RFC3339, strings.TrimPrefix(condition, securityConfigLastRetryConditionPrefix))
			if err == nil {
				return lastRetry
			}
		}
	}
	return time.Time{}
}

func (r *SecurityconfigReconciler) securityConfigRetryConditions(retryCount int, lastRetry time.Time) []string {
	conditions := []string{fmt.Sprintf("%s%d", securityConfigRetryConditionPrefix, retryCount)}
	if !lastRetry.IsZero() {
		conditions = append(conditions, fmt.Sprintf("%s%s", securityConfigLastRetryConditionPrefix, lastRetry.Format(time.RFC3339)))
	}
	return conditions
}

func (r *SecurityconfigReconciler) currentSecurityConfigRetryConditions() []string {
	componentStatus, found := r.securityConfigComponentStatus()
	if found {
		return componentStatus.Conditions
	}
	return nil
}

func securityConfigRetryDelay(retryCount int) time.Duration {
	delay := securityConfigInitialRetryDelay
	for i := 0; i < retryCount; i++ {
		delay *= 2
		if delay >= securityConfigMaxRetryDelay {
			return securityConfigMaxRetryDelay
		}
	}
	return delay
}
