package reconcilers

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-logr/logr"
	opsterv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconciler"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	checksumAnnotation = "securityconfig/checksum"

	adminCert = "/certs/tls.crt"
	adminKey  = "/certs/tls.key"
	caCert    = "/certs/ca.crt"

	SecurityAdminBaseCmdTmpl = `ADMIN=/usr/share/opensearch/plugins/opensearch-security/tools/securityadmin.sh;
chmod +x $ADMIN;
until curl -k --silent https://%s:%v;
do
echo 'Waiting to connect to the cluster'; sleep 20;
done;`

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
	"whitelist.yml":      "whitelist",
	"audit.yml":          "audit",
	"allowlist.yml":      "allowlist",
	"config.yml":         "config",
}

type SecurityconfigReconciler struct {
	client            k8s.K8sClient
	recorder          record.EventRecorder
	reconcilerContext *ReconcilerContext
	instance          *opsterv1.OpenSearchCluster
	logger            logr.Logger
}

func NewSecurityconfigReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	reconcilerContext *ReconcilerContext,
	instance *opsterv1.OpenSearchCluster,
	opts ...reconciler.ResourceReconcilerOption,
) *SecurityconfigReconciler {
	return &SecurityconfigReconciler{
		client:            k8s.NewK8sClient(client, ctx, append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "securityconfig")))...),
		reconcilerContext: reconcilerContext,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx),
	}
}

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
		updateErr := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
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
		updateErr := r.client.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(r.instance), func(instance *opsterv1.OpenSearchCluster) {
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
	if err == nil {
		value, exists := job.Annotations[checksumAnnotation]
		if exists && value == checksumval {
			// Nothing to do, current securityconfig already applied
			return ctrl.Result{}, nil
		}
		// Delete old job
		r.logger.Info("Deleting old update job")
		err := r.client.DeleteJob(&job)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// If the cluster has not yet initialized or
	// securityconfig secret was not passed, build the command to apply all yml files
	if !r.instance.Status.Initialized || len(cmdArg) == 0 {
		clusterHostName := BuildClusterSvcHostName(r.instance)
		httpPort, securityConfigPort, securityconfigPath := helpers.VersionCheck(r.instance)
		cmdArg = fmt.Sprintf(SecurityAdminBaseCmdTmpl, clusterHostName, httpPort) +
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
func BuildCmdArg(instance *opsterv1.OpenSearchCluster, secret *corev1.Secret, log logr.Logger) string {
	clusterHostName := BuildClusterSvcHostName(instance)
	httpPort, securityConfigPort, securityconfigPath := helpers.VersionCheck(instance)

	arg := fmt.Sprintf(SecurityAdminBaseCmdTmpl, clusterHostName, httpPort)

	// Get the list of yml files and sort them
	// This will ensure commands are always generated in the same order
	// Needed for tests as well to compare actual and expected command
	keys := make([]string, 0, len(secret.Data))
	for k := range secret.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		filePath := fmt.Sprintf("%s/%s", securityconfigPath, k)
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

func (r *SecurityconfigReconciler) DeleteResources() (ctrl.Result, error) {
	result := reconciler.CombinedResult{}
	return result.Result, result.Err
}

func (r *SecurityconfigReconciler) securityconfigSubpaths(instance *opsterv1.OpenSearchCluster, secret *corev1.Secret) error {
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
func BuildClusterSvcHostName(instance *opsterv1.OpenSearchCluster) string {
	return fmt.Sprintf("%s.svc.%s", builders.DnsOfService(instance), helpers.ClusterDnsBase())
}
