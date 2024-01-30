package k8s

import (
	"context"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/cisco-open/operator-tools/pkg/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// An abstraction over the kubernetes client to make testing with mocks easier and to hide some repeating boilerplate code
// Note: If you add a method here you need to tun `make genmocks` to update the mock
type K8sClient interface {
	GetSecret(name, namespace string) (corev1.Secret, error)
	CreateSecret(secret *corev1.Secret) (*ctrl.Result, error)
	GetJob(name, namespace string) (batchv1.Job, error)
	CreateJob(job *batchv1.Job) (*ctrl.Result, error)
	DeleteJob(job *batchv1.Job) error
	GetConfigMap(name, namespace string) (corev1.ConfigMap, error)
	CreateConfigMap(cm *corev1.ConfigMap) (*ctrl.Result, error)
	GetStatefulSet(name, namespace string) (appsv1.StatefulSet, error)
	DeleteStatefulSet(sts *appsv1.StatefulSet, orphan bool) error
	ListStatefulSets(listOptions ...client.ListOption) (appsv1.StatefulSetList, error)
	GetDeployment(name, namespace string) (appsv1.Deployment, error)
	CreateDeployment(deployment *appsv1.Deployment) (*ctrl.Result, error)
	DeleteDeployment(deployment *appsv1.Deployment, orphan bool) error
	GetService(name, namespace string) (corev1.Service, error)
	CreateService(svc *corev1.Service) (*ctrl.Result, error)
	GetOpenSearchCluster(name, namespace string) (opsterv1.OpenSearchCluster, error)
	UpdateOpenSearchClusterStatus(key client.ObjectKey, f func(*opsterv1.OpenSearchCluster)) error
	UdateObjectStatus(instance client.Object, f func(client.Object)) error
	ReconcileResource(runtime.Object, reconciler.DesiredState) (*ctrl.Result, error)
	GetPod(name, namespace string) (corev1.Pod, error)
	DeletePod(pod *corev1.Pod) error
	ListPods(listOptions *client.ListOptions) (corev1.PodList, error)
	GetPVC(name, namespace string) (corev1.PersistentVolumeClaim, error)
	UpdatePVC(pvc *corev1.PersistentVolumeClaim) error
	ListPVCs(listOptions *client.ListOptions) (corev1.PersistentVolumeClaimList, error)
	Scheme() *runtime.Scheme
	Context() context.Context
}

type K8sClientImpl struct {
	reconciler.ResourceReconciler
	client.Client
	ctx context.Context
}

func NewK8sClient(client client.Client, ctx context.Context, opts ...reconciler.ResourceReconcilerOption) K8sClientImpl {
	return K8sClientImpl{
		Client: client,
		ResourceReconciler: reconciler.NewReconcilerWith(client,
			append(opts, reconciler.WithLog(log.FromContext(ctx)))...),
		ctx: ctx,
	}
}

func (c K8sClientImpl) GetSecret(name, namespace string) (corev1.Secret, error) {
	secret := corev1.Secret{}
	err := c.Get(c.ctx, client.ObjectKey{Name: name, Namespace: namespace}, &secret)
	return secret, err
}

func (c K8sClientImpl) CreateSecret(secret *corev1.Secret) (*ctrl.Result, error) {
	return c.ReconcileResource(secret, reconciler.StatePresent)
}

func (c K8sClientImpl) GetJob(name, namespace string) (batchv1.Job, error) {
	job := batchv1.Job{}
	err := c.Get(c.ctx, client.ObjectKey{Name: name, Namespace: namespace}, &job)
	return job, err
}

func (c K8sClientImpl) DeleteJob(job *batchv1.Job) error {
	opts := client.DeleteOptions{}
	// Add this so pods of the job are deleted as well, otherwise they would remain as orphaned pods
	client.PropagationPolicy(metav1.DeletePropagationForeground).ApplyToDelete(&opts)
	err := c.Delete(c.ctx, job, &opts)
	if err != nil {
		return err
	}
	// Make sure job is completely deleted (when r.Delete returns deletion sometimes is not yet complete)
	_, err = c.ReconcileResource(job, reconciler.StateAbsent)
	if err != nil {
		return err
	}
	return nil
}

func (c K8sClientImpl) CreateJob(job *batchv1.Job) (*ctrl.Result, error) {
	return c.ReconcileResource(job, reconciler.StatePresent)
}

func (c K8sClientImpl) GetConfigMap(name, namespace string) (corev1.ConfigMap, error) {
	cm := corev1.ConfigMap{}
	err := c.Get(c.ctx, client.ObjectKey{Name: name, Namespace: namespace}, &cm)
	return cm, err
}

func (c K8sClientImpl) CreateConfigMap(cm *corev1.ConfigMap) (*ctrl.Result, error) {
	return c.ReconcileResource(cm, reconciler.StatePresent)
}

func (c K8sClientImpl) GetStatefulSet(name, namespace string) (appsv1.StatefulSet, error) {
	sts := appsv1.StatefulSet{}
	err := c.Get(c.ctx, client.ObjectKey{Name: name, Namespace: namespace}, &sts)
	return sts, err
}

func (c K8sClientImpl) DeleteStatefulSet(sts *appsv1.StatefulSet, orphan bool) error {
	opts := client.DeleteOptions{}
	if orphan {
		// Orphan any pods so that they are not deleted
		client.PropagationPolicy(metav1.DeletePropagationOrphan).ApplyToDelete(&opts)
	} else {
		// Add this so pods of the sts are deleted as well, otherwise they would remain as orphaned pods
		client.PropagationPolicy(metav1.DeletePropagationForeground).ApplyToDelete(&opts)
	}
	return c.Delete(c.ctx, sts, &opts)
}

func (c K8sClientImpl) ListStatefulSets(listOptions ...client.ListOption) (appsv1.StatefulSetList, error) {
	list := appsv1.StatefulSetList{}
	err := c.List(c.ctx, &list, listOptions...)
	return list, err
}

func (c K8sClientImpl) GetDeployment(name, namespace string) (appsv1.Deployment, error) {
	deployment := appsv1.Deployment{}
	err := c.Get(c.ctx, client.ObjectKey{Name: name, Namespace: namespace}, &deployment)
	return deployment, err
}

func (c K8sClientImpl) CreateDeployment(deployment *appsv1.Deployment) (*ctrl.Result, error) {
	return c.ReconcileResource(deployment, reconciler.StatePresent)
}

func (c K8sClientImpl) DeleteDeployment(deployment *appsv1.Deployment, orphan bool) error {
	opts := client.DeleteOptions{}
	if orphan {
		// Orphan any pods so that they are not deleted
		client.PropagationPolicy(metav1.DeletePropagationOrphan).ApplyToDelete(&opts)
	} else {
		// Add this so pods of the sts are deleted as well, otherwise they would remain as orphaned pods
		client.PropagationPolicy(metav1.DeletePropagationForeground).ApplyToDelete(&opts)
	}
	return c.Delete(c.ctx, deployment, &opts)
}

func (c K8sClientImpl) GetService(name, namespace string) (corev1.Service, error) {
	svc := corev1.Service{}
	err := c.Get(c.ctx, client.ObjectKey{Name: name, Namespace: namespace}, &svc)
	return svc, err
}

func (c K8sClientImpl) CreateService(svc *corev1.Service) (*ctrl.Result, error) {
	return c.ReconcileResource(svc, reconciler.StatePresent)
}

func (c K8sClientImpl) GetOpenSearchCluster(name, namespace string) (opsterv1.OpenSearchCluster, error) {
	cluster := opsterv1.OpenSearchCluster{}
	err := c.Get(c.ctx, client.ObjectKey{Name: name, Namespace: namespace}, &cluster)
	return cluster, err
}

func (c K8sClientImpl) UpdateOpenSearchClusterStatus(key client.ObjectKey, f func(*opsterv1.OpenSearchCluster)) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		instance := opsterv1.OpenSearchCluster{}
		if err := c.Get(c.ctx, key, &instance); err != nil {
			return err
		}
		f(&instance)
		return c.Status().Update(c.ctx, &instance)
	})
}

// UpdateStatus for a generic kubernetes object. f should cast to specific type (e.g. `role := instance.(OpenSearchRole)`)
func (c K8sClientImpl) UdateObjectStatus(instance client.Object, f func(client.Object)) error {
	key := client.ObjectKeyFromObject(instance)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := c.Get(c.ctx, key, instance); err != nil {
			return err
		}
		f(instance)
		return c.Status().Update(c.ctx, instance)
	})
}

func (c K8sClientImpl) ReconcileResource(object runtime.Object, state reconciler.DesiredState) (*ctrl.Result, error) {
	return c.ResourceReconciler.ReconcileResource(object, state)
}

func (c K8sClientImpl) GetPod(name, namespace string) (corev1.Pod, error) {
	pod := corev1.Pod{}
	err := c.Get(c.ctx, client.ObjectKey{Name: name, Namespace: namespace}, &pod)
	return pod, err
}

func (c K8sClientImpl) DeletePod(pod *corev1.Pod) error {
	return c.Delete(c.ctx, pod)
}

func (c K8sClientImpl) ListPods(listOptions *client.ListOptions) (corev1.PodList, error) {
	list := corev1.PodList{}
	err := c.List(c.ctx, &list, listOptions)
	return list, err
}

func (c K8sClientImpl) GetPVC(name, namespace string) (corev1.PersistentVolumeClaim, error) {
	pvc := corev1.PersistentVolumeClaim{}
	err := c.Get(c.ctx, client.ObjectKey{Name: name, Namespace: namespace}, &pvc)
	return pvc, err
}

func (c K8sClientImpl) UpdatePVC(pvc *corev1.PersistentVolumeClaim) error {
	return c.Update(c.ctx, pvc)
}

func (c K8sClientImpl) ListPVCs(listOptions *client.ListOptions) (corev1.PersistentVolumeClaimList, error) {
	list := corev1.PersistentVolumeClaimList{}
	err := c.List(c.ctx, &list, listOptions)
	return list, err
}

func (c K8sClientImpl) Scheme() *runtime.Scheme {
	return c.Client.Scheme()
}

func (c K8sClientImpl) Context() context.Context {
	return c.ctx
}

// Validate K8sClientImpl implements the interface
var _ K8sClient = (*K8sClientImpl)(nil)
