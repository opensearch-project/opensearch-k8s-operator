package reconcilers

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io/ioutil"
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

	var configSecretName string
	adminCertName := r.determineAdminSecret()
	namespace := r.instance.Namespace
	clusterName := r.instance.Name
	//Checking if Security Config values are empty and creates a default-securityconfig secret
	if r.instance.Spec.Security.Config == nil || r.instance.Spec.Security.Config.SecurityconfigSecret.Name == "" {
		r.logger.Info(clusterName + "-default-securityconfig is being created")
		SecurityConfigSecretName := clusterName + "-default-securityconfig"
		//Basic SecurityConfigSecret secret with default settings
		SecurityConfigSecret := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: SecurityConfigSecretName, Namespace: namespace}, Type: corev1.SecretTypeOpaque, StringData: make(map[string]string)}
		if err := r.Get(r.ctx, client.ObjectKey{Name: SecurityConfigSecretName, Namespace: namespace}, &SecurityConfigSecret); err == nil {
			r.logger.Info(clusterName + "-default-securityconfig secret exists")
		} else {
			r.logger.Info("creating " + clusterName + "-default-securityconfig secret")
			r.recorder.Event(r.instance, "Normal", "Security", fmt.Sprintf("Creating securityconfig secret to %s/%s", r.instance.Namespace, r.instance.Name))
			//Reads all securityconfig files and adds them to secret Stringdata
			files, err := ioutil.ReadDir("./helperfiles/defaultsecurityconfigs/")
			if err != nil {
				r.logger.Info("Failed to read directory helperfiles/defaultsecurityconfigs")
				//panic(err)
			}
			for _, f := range files {
				fileBytes, err := ioutil.ReadFile("./helperfiles/defaultsecurityconfigs/" + f.Name())
				if err != nil {
					r.logger.Info("Failed to add " + f.Name() + clusterName + "-default-securityconfig secret")
					r.recorder.Event(r.instance, "Warning", "Security", fmt.Sprintf("Failed to add %s %s default-securityconfig secret", f.Name(), clusterName))
					//panic(err)
				}
				SecurityConfigSecret.StringData[f.Name()] = string(fileBytes)
			}
			if err := ctrl.SetControllerReference(r.instance, &SecurityConfigSecret, r.Client.Scheme()); err != nil {
				return ctrl.Result{}, err
			}
			//r.Create(r.ctx, &SecurityConfigSecret)
			if err := r.Create(r.ctx, &SecurityConfigSecret); err != nil {
				r.logger.Error(err, "Failed to create default"+clusterName+"-default-securityconfig secret")
				r.recorder.Event(r.instance, "Warning", "Security", fmt.Sprintf("Failed to create default %s default-securityconfig secret", clusterName))
				return ctrl.Result{}, err
			}

		}
		configSecretName = clusterName + "-default-securityconfig"
	} else {
		//Use a user passed value of SecurityconfigSecret name
		configSecretName = r.instance.Spec.Security.Config.SecurityconfigSecret.Name
	}

	jobName := clusterName + "-securityconfig-update"

	if adminCertName == "" {
		r.logger.Info("Cluster is running with demo certificates.")
		r.recorder.Event(r.instance, "Warning", "Security", fmt.Sprintf("Cluster is running with demo certificates"))
		return ctrl.Result{}, nil
	}

	// Wait for secret to be available
	configSecret := corev1.Secret{}
	if err := r.Get(r.ctx, client.ObjectKey{Name: configSecretName, Namespace: namespace}, &configSecret); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Info(fmt.Sprintf("Waiting for secret '%s' that contains the securityconfig to be created", configSecretName))
			r.recorder.Event(r.instance, "Info", "Security", fmt.Sprintf("Waiting for secret '%s' that contains the securityconfig to be created", configSecretName))
			return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 10}, nil
		}
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
	r.recorder.Event(r.instance, "Info", "Security", fmt.Sprintf("Starting securityconfig update job"))

	job = builders.NewSecurityconfigUpdateJob(
		r.instance,
		jobName,
		namespace,
		checksum,
		adminCertName,
		clusterName,
		r.reconcilerContext.Volumes,
		r.reconcilerContext.VolumeMounts,
	)
	if err := ctrl.SetControllerReference(r.instance, &job, r.Client.Scheme()); err != nil {
		return ctrl.Result{}, err
	}
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
