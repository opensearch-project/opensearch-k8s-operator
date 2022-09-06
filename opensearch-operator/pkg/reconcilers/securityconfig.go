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
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/builders"
	"opensearch.opster.io/pkg/helpers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	checksumAnnotation = "securityconfig/checksum"
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
	annotations := map[string]string{"cluster-name": r.instance.GetName()}
	var configSecretName string
	var checksumval string
	adminCertName := r.determineAdminSecret()
	namespace := r.instance.Namespace
	clusterName := r.instance.Name
	jobName := clusterName + "-securityconfig-update"
	if adminCertName == "" {
		r.logger.Info("Cluster is running with demo certificates.")
		r.recorder.AnnotatedEventf(r.instance, annotations, "Warning", "Security", "Notice - Cluster is running with demo certificates")
		return ctrl.Result{}, nil
	}
	//Checking if Security Config values are empty and creates a default-securityconfig secret
	if r.instance.Spec.Security.Config != nil && r.instance.Spec.Security.Config.SecurityconfigSecret.Name != "" {
		//Use a user passed value of SecurityconfigSecret name
		configSecretName = r.instance.Spec.Security.Config.SecurityconfigSecret.Name
		// Wait for secret to be available
		configSecret := corev1.Secret{}
		if err := r.Get(r.ctx, client.ObjectKey{Name: configSecretName, Namespace: namespace}, &configSecret); err != nil {
			if apierrors.IsNotFound(err) {
				r.logger.Info(fmt.Sprintf("Waiting for secret '%s' that contains the securityconfig to be created", configSecretName))
				r.recorder.AnnotatedEventf(r.instance, annotations, "Info", "Security", "Notice - Waiting for secret '%s' that contains the securityconfig to be created", configSecretName)
				return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 10}, nil
			}
			return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 10}, err
		}

		// Check config secret OwnerReferrences
		if configSecret.OwnerReferences == nil {
			if err := ctrl.SetControllerReference(r.instance, &configSecret, r.Client.Scheme()); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Update(r.ctx, &configSecret); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Calculate checksum and check for changes
		var checksumerr error
		checksumval, checksumerr = checksum(configSecret.Data)
		if checksumerr != nil {
			return ctrl.Result{}, checksumerr
		}
		if err := r.securityconfigSubpaths(r.instance, &configSecret); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		r.logger.Info("Not passed any SecurityconfigSecret")
	}

	job := batchv1.Job{}
	if err := r.Get(r.ctx, client.ObjectKey{Name: jobName, Namespace: namespace}, &job); err == nil {
		value, exists := job.ObjectMeta.Annotations[checksumAnnotation]
		if exists && value == checksumval {
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
	r.recorder.AnnotatedEventf(r.instance, annotations, "Normal", "Security", "Starting to securityconfig update job")
	job = builders.NewSecurityconfigUpdateJob(
		r.instance,
		jobName,
		namespace,
		checksumval,
		adminCertName,
		clusterName,
		r.reconcilerContext.Volumes,
		r.reconcilerContext.VolumeMounts,
	)
	if err := ctrl.SetControllerReference(r.instance, &job, r.Client.Scheme()); err != nil {
		return ctrl.Result{}, err
	}
	_, err := r.ReconcileResource(&job, reconciler.StateCreated)
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

func (r *SecurityconfigReconciler) determineAdminSecret() string {
	if r.instance.Spec.Security.Config != nil && r.instance.Spec.Security.Config.AdminSecret.Name != "" {
		return r.instance.Spec.Security.Config.AdminSecret.Name
	} else if r.instance.Spec.Security.Tls != nil && r.instance.Spec.Security.Tls.Transport != nil && r.instance.Spec.Security.Tls.Transport.Generate {
		return fmt.Sprintf("%s-admin-cert", r.instance.Name)
	} else {
		return ""
	}
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
	_, securityconfigPath := helpers.VersionCheck(instance)
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
