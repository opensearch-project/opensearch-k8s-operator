package reconcilers

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"sort"
	"time"

	"github.com/banzaicloud/operator-tools/pkg/reconciler"
	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/builders"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	checksumAnnotation = "securityconfig/checksum"
	securityconfigPath = "/usr/share/opensearch/plugins/opensearch-security/securityconfig"
)

type SecurityconfigReconciler struct {
	reconciler.ResourceReconciler
	client.Client
	ctx               context.Context
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
		Client: client,
		ResourceReconciler: reconciler.NewReconcilerWith(client,
			append(opts, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "securityconfig")))...),
		ctx:               ctx,
		reconcilerContext: reconcilerContext,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx),
	}
}

func (r *SecurityconfigReconciler) Reconcile() (ctrl.Result, error) {
	if r.instance.Spec.Security == nil || r.instance.Spec.Security.Config == nil || r.instance.Spec.Security.Config.SecurityconfigSecret.Name == "" {
		return ctrl.Result{}, nil
	}
	namespace := r.instance.Namespace
	clusterName := r.instance.Name
	jobName := clusterName + "-securityconfig-update"
	configSecretName := r.instance.Spec.Security.Config.SecurityconfigSecret.Name
	adminCertName := r.determineAdminSecret()

	if adminCertName == "" {
		r.logger.Info("Cluster is running with demo certificates. Skipping securityconfig")
		return ctrl.Result{}, nil
	}

	// Wait for secret to be available
	configSecret := corev1.Secret{}
	if err := r.Get(r.ctx, client.ObjectKey{Name: configSecretName, Namespace: namespace}, &configSecret); err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 10}, err
	}

	// Calculate checksum and check for changes
	checksum, err := checksum(configSecret.Data)
	if err != nil {
		return ctrl.Result{}, err
	}
	job := batchv1.Job{}
	if err := r.Get(r.ctx, client.ObjectKey{Name: jobName, Namespace: namespace}, &job); err == nil {
		value, exists := job.ObjectMeta.Annotations[checksumAnnotation]
		if exists && value == checksum {
			// Nothing to do, current securityconfig already applied
			return ctrl.Result{}, nil
		}
		// Delete old job
		r.logger.Info("Deleting old update job")
		opts := client.DeleteOptions{}
		// Add this so pods of the job are deleted as well, otherwise they would remain as orphaned pods
		client.PropagationPolicy(metav1.DeletePropagationForeground).ApplyToDelete(&opts)
		err = r.Delete(r.ctx, &job, &opts)
		if err != nil {
			return ctrl.Result{}, err
		}
		// Make sure job is completely deleted (when r.Delete returns deletion sometimes is not yet complete)
		_, err = r.ReconcileResource(&job, reconciler.StateAbsent)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	r.logger.Info("Starting securityconfig update job")
	job = r.job(jobName, namespace, checksum, adminCertName)
	_, err = r.ReconcileResource(&job, reconciler.StateCreated)
	return ctrl.Result{}, err
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

func (r *SecurityconfigReconciler) job(jobName string, namespace string, checksum string, adminCertName string) batchv1.Job {
	dns := builders.DnsOfService(r.instance)
	adminCert := "/certs/tls.crt"
	adminKey := "/certs/tls.key"
	caCert := "/certs/ca.crt"
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	volumes = append(volumes, corev1.Volume{
		Name:         "securityconfig",
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: r.instance.Spec.Security.Config.SecurityconfigSecret.Name}},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "securityconfig",
		MountPath: "/securityconfig",
	})
	volumes = append(volumes, corev1.Volume{
		Name:         "admin-cert",
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: adminCertName}},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "admin-cert",
		MountPath: "/certs",
	})

	arg := "ADMIN=/usr/share/opensearch/plugins/opensearch-security/tools/securityadmin.sh;" +
		"chmod +x $ADMIN;" +
		"count=0;" +
		fmt.Sprintf("until $ADMIN -cacert %s -cert %s -key %s -cd /securityconfig/ -icl -nhnv -h %s.svc.cluster.local -p 9300 || (( count++ >= 20 )); do", caCert, adminCert, adminKey, dns) +
		"  sleep 20; " +
		"done"
	annotations := map[string]string{
		checksumAnnotation: checksum,
	}
	terminationGracePeriodSeconds := int64(5)
	backoffLimit := int32(0)

	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: namespace, Annotations: annotations},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Name: jobName},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{{
						Name:         "updater",
						Image:        builders.DockerImageForCluster(r.instance),
						Command:      []string{"/bin/bash", "-c"},
						Args:         []string{arg},
						VolumeMounts: volumeMounts,
					}},
					Volumes:       volumes,
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
}

func (r *SecurityconfigReconciler) determineAdminSecret() string {
	if r.instance.Spec.Security.Config.AdminSecret.Name != "" {
		return r.instance.Spec.Security.Config.AdminSecret.Name
	} else if r.instance.Spec.Security.Tls.Transport.Generate {
		return fmt.Sprintf("%s-admin-cert", r.instance.Name)
	} else {
		return ""
	}
}
