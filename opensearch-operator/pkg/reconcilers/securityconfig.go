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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/builders"
	"opensearch.opster.io/pkg/config"
	"opensearch.opster.io/pkg/helpers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	checksumAnnotation = "securityconfig/checksum"
	defaultVolumeName  = "defaultsecurityconfig"
	providedVolumeName = "securityconfig"
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

	if r.instance.Spec.Security == nil {
		return ctrl.Result{}, nil
	}

	adminCertName := r.determineAdminSecret()
	namespace := r.instance.Namespace
	clusterName := r.instance.Name
	var overriddenKeys []string
	var providedSecret *corev1.Secret

	if r.instance.Spec.Security.Config != nil && r.instance.Spec.Security.Config.SecurityconfigSecret.Name != "" {
		providedSecret = &corev1.Secret{}
		if err := r.Get(r.ctx, types.NamespacedName{
			Name:      r.instance.Spec.Security.Config.SecurityconfigSecret.Name,
			Namespace: namespace,
		}, providedSecret); err != nil {
			if apierrors.IsNotFound(err) {
				r.logger.Info(fmt.Sprintf("%s not found ", r.instance.Spec.Security.Config.SecurityconfigSecret.Name))
				r.recorder.Event(r.instance, "Normal", "SecretNotFound", "configured security config secret not found")
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: time.Second * 10,
				}, nil
			}
			return ctrl.Result{}, err
		}
		for key := range providedSecret.Data {
			overriddenKeys = append(overriddenKeys, key)
		}

		r.reconcilerContext.Volumes = append(r.reconcilerContext.Volumes, corev1.Volume{
			Name: providedVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: providedSecret.Name,
				},
			},
		})

		r.calculateSecurityconfigSubpaths(providedVolumeName, providedSecret)
	}

	defaultSecurityConfigSecretName := clusterName + "-default-securityconfig"
	defaultSecurityConfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultSecurityConfigSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{},
	}

	for key, value := range config.DefaultSecurityConfig {
		if !helpers.ContainsString(overriddenKeys, key) {
			defaultSecurityConfigSecret.Data[key] = *value
		}
	}

	if len(defaultSecurityConfigSecret.Data) > 0 {
		if err := r.Create(r.ctx, defaultSecurityConfigSecret); err != nil {
			r.logger.Error(err, fmt.Sprintf("failed to create %s secret", defaultSecurityConfigSecret.Name))
			r.recorder.Event(r.instance, "Warning", "Security", fmt.Sprintf("Failed to create default %s default-securityconfig secret", clusterName))
			return ctrl.Result{}, err
		}

		r.reconcilerContext.Volumes = append(r.reconcilerContext.Volumes, corev1.Volume{
			Name: defaultVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: defaultSecurityConfigSecretName,
				},
			},
		})

		r.calculateSecurityconfigSubpaths(defaultVolumeName, defaultSecurityConfigSecret)
	}

	jobName := clusterName + "-securityconfig-update"

	if adminCertName == "" {
		r.logger.Info("Cluster is running with demo certificates.")
		r.recorder.Event(r.instance, "Warning", "Security", "Notice - Cluster is running with demo certificates")
		return ctrl.Result{}, nil
	}

	// Wait for secrets
	if len(defaultSecurityConfigSecret.Data) > 0 {
		if err := r.Get(r.ctx, types.NamespacedName{
			Name:      defaultSecurityConfigSecretName,
			Namespace: namespace,
		}, &corev1.Secret{}); err != nil {
			if apierrors.IsNotFound(err) {
				r.logger.Info(fmt.Sprintf("%s not found ", defaultSecurityConfigSecretName))
				r.recorder.Event(r.instance, "Normal", "SecretNotFound", "default security config secret not found")
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: time.Second * 10,
				}, nil
			}
			return ctrl.Result{}, err
		}
	}

	// Calculate checksum and check for changes
	secretData := []map[string][]byte{
		defaultSecurityConfigSecret.Data,
	}

	if providedSecret != nil {
		secretData = append(secretData, providedSecret.Data)
	}

	checksumValue, err := checksum(secretData...)
	if err != nil {
		return ctrl.Result{}, err
	}

	job := batchv1.Job{}
	if err := r.Get(r.ctx, client.ObjectKey{Name: jobName, Namespace: namespace}, &job); err == nil {
		value, exists := job.ObjectMeta.Annotations[checksumAnnotation]
		if exists && value == checksumValue {
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
	r.recorder.Event(r.instance, "Normal", "Security", "Starting to securityconfig update job")
	job = builders.NewSecurityconfigUpdateJob(
		r.instance,
		jobName,
		namespace,
		checksumValue,
		adminCertName,
		clusterName,
		len(defaultSecurityConfigSecret.Data) > 0,
		overriddenKeys,
		r.reconcilerContext.Volumes,
		r.reconcilerContext.VolumeMounts,
	)
	if err := ctrl.SetControllerReference(r.instance, &job, r.Client.Scheme()); err != nil {
		return ctrl.Result{}, err
	}
	_, err = r.ReconcileResource(&job, reconciler.StateCreated)
	return ctrl.Result{}, err
}

func checksum(data ...map[string][]byte) (string, error) {
	combined := map[string][]byte{}
	for _, input := range data {
		for k, v := range input {
			combined[k] = v
		}
	}

	hash := sha1.New()
	keys := make([]string, 0, len(data))
	for k := range combined {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		_, err := hash.Write([]byte(key))
		if err != nil {
			return "", err
		}
		value := combined[key]
		_, err = hash.Write(value)
		if err != nil {
			return "", err
		}
	}
	return base64.StdEncoding.EncodeToString(hash.Sum(nil)), nil
}

func (r *SecurityconfigReconciler) determineAdminSecret() string {
	if r.instance.Spec.Security.Config != nil && r.instance.Spec.Security.Config.AdminSecret.Name != "" {
		return r.instance.Spec.Security.Config.AdminSecret.Name
	} else if r.instance.Spec.Security.Tls != nil && r.instance.Spec.Security.Tls.Transport != nil && r.instance.Spec.Security.Tls.Transport.Generate {
		return fmt.Sprintf("%s-admin-cert", r.instance.Name)
	} else {
		return ""
	}
}

func (r *SecurityconfigReconciler) calculateSecurityconfigSubpaths(volumeName string, secret *corev1.Secret) {
	keys := make([]string, 0, len(secret.Data))
	for k := range secret.Data {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	for _, k := range keys {
		r.reconcilerContext.VolumeMounts = append(r.reconcilerContext.VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: fmt.Sprintf("/usr/share/opensearch/plugins/opensearch-security/securityconfig/%s", k),
			SubPath:   k,
			ReadOnly:  true,
		})
	}
}

func (r *SecurityconfigReconciler) DeleteResources() (ctrl.Result, error) {
	result := reconciler.CombinedResult{}
	return result.Result, result.Err
}
