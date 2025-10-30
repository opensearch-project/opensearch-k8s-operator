// Copyright © 2020 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reconciler

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/go-logr/logr"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/patch"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/types"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/utils"
)

const (
	DefaultRecreateRequeueDelay int32              = 10
	StateCreated                StaticDesiredState = "Created"
	StateAbsent                 StaticDesiredState = "Absent"
	StatePresent                StaticDesiredState = "Present"
)

var DefaultRecreateEnabledGroupKinds = []schema.GroupKind{
	{Group: "", Kind: "Service"},
	{Group: "apps", Kind: "StatefulSet"},
	{Group: "apps", Kind: "DaemonSet"},
	{Group: "apps", Kind: "Deployment"},
}

type DesiredState interface {
	BeforeUpdate(current, desired runtime.Object) error
	BeforeCreate(desired runtime.Object) error
	BeforeDelete(current runtime.Object) error
}

type DesiredStateShouldCreate interface {
	ShouldCreate(desired runtime.Object) (bool, error)
}

type DesiredStateShouldUpdate interface {
	ShouldUpdate(current, desired runtime.Object) (bool, error)
}

type DesiredStateShouldDelete interface {
	ShouldDelete(desired runtime.Object) (bool, error)
}

type DesiredStateWithDeleteOptions interface {
	GetDeleteOptions() []client.DeleteOption
}

type DesiredStateWithCreateOptions interface {
	GetCreateOptions() []client.CreateOption
}

type DesiredStateWithUpdateOptions interface {
	GetUpdateOptions() []client.UpdateOption
}

type DesiredStateWithStaticState interface {
	DesiredState() StaticDesiredState
}

type DesiredStateWithGetter interface {
	GetDesiredState() DesiredState
}

type ResourceReconciler interface {
	CreateIfNotExist(runtime.Object, DesiredState) (created bool, object runtime.Object, err error)
	ReconcileResource(runtime.Object, DesiredState) (*reconcile.Result, error)
}

type StaticDesiredState string

func (s StaticDesiredState) BeforeUpdate(current, desired runtime.Object) error {
	return nil
}

func (s StaticDesiredState) BeforeCreate(desired runtime.Object) error {
	return nil
}

func (s StaticDesiredState) BeforeDelete(current runtime.Object) error {
	return nil
}

type DesiredStateHook func(object runtime.Object) error

func (d DesiredStateHook) BeforeUpdate(current, desired runtime.Object) error {
	return d(current)
}

func (d DesiredStateHook) BeforeCreate(desired runtime.Object) error {
	return d(desired)
}

func (d DesiredStateHook) BeforeDelete(current runtime.Object) error {
	return d(current)
}

// GenericResourceReconciler generic resource reconciler
type GenericResourceReconciler struct {
	Log     logr.Logger
	Client  client.Client
	Options ReconcilerOpts
}

type ResourceReconcilerOption func(*ReconcilerOpts)

type RecreateResourceCondition func(kind schema.GroupVersionKind, status metav1.Status) bool

type ErrorMessageCondition func(string) bool

// Recommended to use NewReconcilerWith + ResourceReconcilerOptions
type ReconcilerOpts struct {
	Log    logr.Logger
	Scheme *runtime.Scheme
	// Enable recreating workloads and services when the API server rejects an update
	EnableRecreateWorkloadOnImmutableFieldChange bool
	// Custom log message to help when a workload or service needs to be recreated
	EnableRecreateWorkloadOnImmutableFieldChangeHelp string
	// The delay in seconds to wait before checking back after deleting the resource (10s by default)
	RecreateRequeueDelay *int32
	// List of callbacks evaluated to decide whether a given gvk is enabled to be recreated or not
	RecreateEnabledResourceCondition RecreateResourceCondition
	// Immediately recreate the resource instead of deleting and returning with a requeue
	RecreateImmediately bool
	// Configure the recreate PropagationPolicy. "Orphan" avoids deleting pods simultaneously.
	RecreatePropagationPolicy client.PropagationPolicy
	// Check the update error message contains this substring before recreate. Default: "immutable"
	RecreateErrorMessageSubstring *string
	// Custom logic to decide if an error message indicates a resource should be recreated.
	// Takes precedence over RecreateErrorMessageSubstring if set.
	RecreateErrorMessageCondition ErrorMessageCondition
	// K8s object matcher patch maker implementation
	PatchMaker patch.Maker
	// K8s object matcher patch calculate options
	PatchCalculateOptions []patch.CalculateOption
}

func MatchImmutableNoStatefulSet(errorMessage string) bool {
	if strings.Contains(errorMessage, "immutable") {
		return true
	}
	if strings.Contains(errorMessage, "may not change once set") {
		return true
	}
	return false
}

func MatchImmutableErrorMessages(errorMessage string) bool {
	if strings.Contains(errorMessage, "immutable") {
		return true
	}
	if strings.Contains(errorMessage, "may not change once set") {
		return true
	}
	// StatefulSet is a special case because it has a different error message
	if strings.Contains(errorMessage, "updates to statefulset spec for fields other than") {
		return true
	}
	return false
}

// NewGenericReconciler returns GenericResourceReconciler
// Deprecated, use NewReconcilerWith
func NewGenericReconciler(c client.Client, log logr.Logger, opts ReconcilerOpts) *GenericResourceReconciler {
	if opts.Scheme == nil {
		opts.Scheme = runtime.NewScheme()
		_ = clientgoscheme.AddToScheme(opts.Scheme)
	}
	if opts.RecreateRequeueDelay == nil {
		opts.RecreateRequeueDelay = utils.IntPointer(DefaultRecreateRequeueDelay)
	}
	if opts.RecreateErrorMessageSubstring == nil {
		if opts.RecreateErrorMessageCondition == nil {
			opts.RecreateErrorMessageCondition = MatchImmutableErrorMessages
		} else {
			opts.RecreateErrorMessageSubstring = utils.StringPointer("immutable")
		}
	}
	if opts.RecreateEnabledResourceCondition == nil {
		// only allow a custom set of types and only specific errors
		opts.RecreateEnabledResourceCondition = func(kind schema.GroupVersionKind, status metav1.Status) bool {
			for _, gk := range DefaultRecreateEnabledGroupKinds {
				if gk == kind.GroupKind() {
					return true
				}
			}
			return false
		}
	}
	if len(opts.RecreatePropagationPolicy) == 0 {
		// DO NOT wait until all dependent resources get cleared up
		opts.RecreatePropagationPolicy = client.PropagationPolicy(metav1.DeletePropagationBackground)
	}
	if opts.PatchMaker == nil {
		opts.PatchMaker = patch.DefaultPatchMaker
	}
	if opts.PatchCalculateOptions == nil {
		opts.PatchCalculateOptions = []patch.CalculateOption{patch.IgnoreStatusFields()}
	}
	return &GenericResourceReconciler{
		Log:     log,
		Client:  c,
		Options: opts,
	}
}

func WithLog(log logr.Logger) ResourceReconcilerOption {
	return func(o *ReconcilerOpts) {
		o.Log = log
	}
}

func WithScheme(scheme *runtime.Scheme) ResourceReconcilerOption {
	return func(o *ReconcilerOpts) {
		o.Scheme = scheme
	}
}

func WithEnableRecreateWorkload() ResourceReconcilerOption {
	return func(o *ReconcilerOpts) {
		o.EnableRecreateWorkloadOnImmutableFieldChange = true
	}
}

// Apply the given amount of delay before recreating a resource after it has been removed
func WithRecreateRequeueDelay(delay int32) ResourceReconcilerOption {
	return func(o *ReconcilerOpts) {
		o.RecreateRequeueDelay = utils.IntPointer(delay)
	}
}

// Use this option for the legacy behaviour
func WithRecreateEnabledForAll() ResourceReconcilerOption {
	return func(o *ReconcilerOpts) {
		o.RecreateEnabledResourceCondition = func(_ schema.GroupVersionKind, _ metav1.Status) bool {
			return true
		}
	}
}

// Use this option for the legacy behaviour
func WithRecreateEnabledFor(condition RecreateResourceCondition) ResourceReconcilerOption {
	return func(o *ReconcilerOpts) {
		o.RecreateEnabledResourceCondition = condition
	}
}

// Matches no GVK
func WithRecreateEnabledForNothing() ResourceReconcilerOption {
	return func(o *ReconcilerOpts) {
		o.RecreateEnabledResourceCondition = func(kind schema.GroupVersionKind, status metav1.Status) bool {
			return false
		}
	}
}

// Recreate workloads immediately without waiting for dependents to get GCd
func WithRecreateImmediately() ResourceReconcilerOption {
	return func(o *ReconcilerOpts) {
		o.RecreateImmediately = true
	}
}

// Recreate only if the error message contains the given substring
func WithRecreateErrorMessageSubstring(substring string) ResourceReconcilerOption {
	return func(o *ReconcilerOpts) {
		o.RecreateErrorMessageSubstring = utils.StringPointer(substring)
	}
}

// Recreate only if the error message contains the given substring
func WithRecreateErrorMessageCondition(condition ErrorMessageCondition) ResourceReconcilerOption {
	return func(o *ReconcilerOpts) {
		o.RecreateErrorMessageCondition = condition
	}
}

// Disable checking the error message before recreating resources
func WithRecreateErrorMessageIgnored() ResourceReconcilerOption {
	return func(o *ReconcilerOpts) {
		o.RecreateErrorMessageSubstring = utils.StringPointer("")
	}
}

// Set patch maker implementation
func WithPatchMaker(maker patch.Maker) ResourceReconcilerOption {
	return func(o *ReconcilerOpts) {
		o.PatchMaker = maker
	}
}

// Set patch maker calculate options
func WithPatchCalculateOptions(options ...patch.CalculateOption) ResourceReconcilerOption {
	return func(o *ReconcilerOpts) {
		o.PatchCalculateOptions = options
	}
}

func NewReconcilerWith(client client.Client, opts ...ResourceReconcilerOption) ResourceReconciler {
	options := ReconcilerOpts{
		Log: logr.Discard(),
		EnableRecreateWorkloadOnImmutableFieldChangeHelp: "recreating object on immutable field change has to be enabled explicitly through the reconciler options",
	}
	for _, opt := range opts {
		opt(&options)
	}
	return NewGenericReconciler(client, options.Log, options)
}

// CreateResource creates a resource if it doesn't exist
func (r *GenericResourceReconciler) CreateResource(desired runtime.Object) error {
	_, _, err := r.CreateIfNotExist(desired, nil)
	return err
}

func (r *GenericResourceReconciler) shouldRecreate(sErr *apierrors.StatusError) bool {
	// If a condition function is set, use it
	if r.Options.RecreateErrorMessageCondition != nil {
		return r.Options.RecreateErrorMessageCondition(sErr.ErrStatus.Message)
	}
	// Fall back to substring matching
	return strings.Contains(sErr.ErrStatus.Message, utils.PointerToString(r.Options.RecreateErrorMessageSubstring))
}

// ReconcileResource reconciles various kubernetes types
func (r *GenericResourceReconciler) ReconcileResource(desired runtime.Object, desiredState DesiredState) (*reconcile.Result, error) {
	resourceDetails, gvk, err := r.resourceDetails(desired)
	if err != nil {
		return nil, errors.WrapIf(err, "failed to get resource details")
	}
	log := r.resourceLog(desired, resourceDetails...)
	debugLog := log.V(1)
	traceLog := log.V(3)
	state := desiredState
	if ds, ok := desiredState.(DesiredStateWithStaticState); ok {
		state = ds.DesiredState()
	} else if ds, ok := desiredState.(DesiredStateWithGetter); ok {
		state = ds.GetDesiredState()
	}
	switch state {
	case StateCreated:
		created, _, err := r.CreateIfNotExist(desired, desiredState)
		if err == nil && created {
			return nil, nil
		}
		if err != nil {
			return nil, errors.WrapIfWithDetails(err, "failed to create resource", resourceDetails...)
		}
	case StateAbsent:
		_, err := r.delete(desired, desiredState)
		if err != nil {
			return nil, errors.WrapIfWithDetails(err, "failed to delete resource", resourceDetails...)
		}
	default:
		created, current, err := r.CreateIfNotExist(desired, desiredState)
		if err == nil && created {
			return nil, nil
		}
		if err != nil {
			return nil, errors.WrapIfWithDetails(err, "failed to create resource", resourceDetails...)
		}

		if metaObject, ok := current.(metav1.Object); ok {
			if metaObject.GetDeletionTimestamp() != nil {
				log.Info(fmt.Sprintf("object %s is being deleted, backing off", metaObject.GetSelfLink()))
				return &reconcile.Result{RequeueAfter: time.Second * 2}, nil
			}
			if !created {
				if desiredMetaObject, ok := desired.(metav1.Object); ok {
					base := types.MetaBase{
						Annotations: desiredMetaObject.GetAnnotations(),
						Labels:      desiredMetaObject.GetLabels(),
					}
					if metaObject, ok := current.DeepCopyObject().(metav1.Object); ok {
						merged := base.Merge(metav1.ObjectMeta{
							Labels:      metaObject.GetLabels(),
							Annotations: metaObject.GetAnnotations(),
						})
						desiredMetaObject.SetAnnotations(merged.Annotations)
						desiredMetaObject.SetLabels(merged.Labels)
					}
				}
				if _, ok := metaObject.GetAnnotations()[types.BanzaiCloudManagedComponent]; !ok {
					if desiredMetaObject, ok := desired.(metav1.Object); ok {
						a := desiredMetaObject.GetAnnotations()
						delete(a, types.BanzaiCloudManagedComponent)
						desiredMetaObject.SetAnnotations(a)
					}
				}
			}
		}

		if ds, ok := desiredState.(DesiredStateShouldUpdate); ok {
			should, err := ds.ShouldUpdate(current.DeepCopyObject(), desired.DeepCopyObject())
			if err != nil {
				return nil, err
			}
			if !should {
				return nil, nil
			}
		}

		// last chance to hook into the desired state armed with the knowledge of the current state
		err = desiredState.BeforeUpdate(current, desired)
		if err != nil {
			return nil, errors.WrapIfWithDetails(err, "failed to get desired state dynamically", resourceDetails...)
		}

		patchResult, err := r.Options.PatchMaker.Calculate(current, desired, r.Options.PatchCalculateOptions...)
		if err != nil {
			debugLog.Info("could not match objects", "error", err)
		} else if patchResult.IsEmpty() {
			debugLog.Info("resource is in sync")
			return nil, nil
		} else {
			if gvk.Kind == "Secret" {
				debugLog.Info("resource diff")
			} else {
				debugLog.Info("resource diff", "patch", string(patchResult.Patch))
				traceLog.Info("resource states",
					"current", string(patchResult.Current),
					"modified", string(patchResult.Modified),
					"original", string(patchResult.Original))
			}
		}

		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(desired); err != nil {
			log.Error(err, "Failed to set last applied annotation", "desired", desired)
		}

		metaAccessor := meta.NewAccessor()

		currentResourceVersion, err := metaAccessor.ResourceVersion(current)
		if err != nil {
			return nil, errors.WrapIfWithDetails(err, "failed to access resourceVersion from metadata", resourceDetails...)
		}
		if err := metaAccessor.SetResourceVersion(desired, currentResourceVersion); err != nil {
			return nil, errors.WrapIfWithDetails(err, "failed to set resourceVersion in metadata", resourceDetails...)
		}

		debugLog.Info("updating resource")
		var updateOptions []client.UpdateOption
		if ds, ok := desiredState.(DesiredStateWithUpdateOptions); ok {
			updateOptions = append(updateOptions, ds.GetUpdateOptions()...)
		}
		if err := r.Client.Update(context.TODO(), desired.(client.Object), updateOptions...); err != nil {
			sErr, ok := err.(*apierrors.StatusError)
			if ok && (sErr.ErrStatus.Code == 422 && sErr.ErrStatus.Reason == metav1.StatusReasonInvalid) && r.shouldRecreate(sErr) {
				if r.Options.EnableRecreateWorkloadOnImmutableFieldChange {
					if !r.Options.RecreateEnabledResourceCondition(gvk, sErr.ErrStatus) {
						return nil, errors.WrapIfWithDetails(err, "resource type is not allowed to be recreated", resourceDetails...)
					}
					log.Error(err, "failed to update resource, trying to recreate", resourceDetails...)
					if r.Options.RecreateImmediately {
						err := r.Client.Delete(context.TODO(), current.(client.Object),
							r.Options.RecreatePropagationPolicy,
						)
						if err != nil {
							return nil, errors.WrapIfWithDetails(err, "failed to delete current resource", resourceDetails...)
						}
						if err := metaAccessor.SetResourceVersion(desired, ""); err != nil {
							return nil, errors.WrapIfWithDetails(err, "unable to clear resourceVersion", resourceDetails...)
						}
						created, _, err := r.CreateIfNotExist(desired, desiredState)
						if err == nil {
							if !created {
								return nil, errors.New("resource already exists")
							}
							return nil, nil
						}
						if err != nil {
							return nil, errors.WrapIfWithDetails(err, "failed to recreate resource", resourceDetails...)
						}
					}
					err := r.Client.Delete(context.TODO(), current.(client.Object),
						// wait until all dependent resources get cleared up
						client.PropagationPolicy(metav1.DeletePropagationForeground),
					)
					if err != nil {
						return nil, errors.WrapIfWithDetails(err, "failed to delete current resource", resourceDetails...)
					}
					return &reconcile.Result{
						Requeue:      true,
						RequeueAfter: time.Second * time.Duration(utils.PointerToInt32(r.Options.RecreateRequeueDelay)),
					}, nil
				} else {
					return nil, errors.WrapIf(sErr, r.Options.EnableRecreateWorkloadOnImmutableFieldChangeHelp)
				}
			}
			return nil, errors.WrapIfWithDetails(err, "updating resource failed", resourceDetails...)
		}
		debugLog.Info("resource updated")
	}
	return nil, nil
}

func (r *GenericResourceReconciler) fromDesired(desired runtime.Object) (runtime.Object, error) {
	if _, ok := desired.(*unstructured.Unstructured); ok {
		if r.Options.Scheme != nil {
			object, err := r.Options.Scheme.New(desired.GetObjectKind().GroupVersionKind())
			if err == nil {
				return object, nil
			}
			r.Log.V(2).Info("unable to detect correct type for the resource, falling back to unstructured")
		}
		current := &unstructured.Unstructured{}
		desiredGVK := desired.GetObjectKind()
		current.SetKind(desiredGVK.GroupVersionKind().Kind)
		current.SetAPIVersion(desiredGVK.GroupVersionKind().GroupVersion().String())
		return current, nil
	}
	return reflect.New(reflect.Indirect(reflect.ValueOf(desired)).Type()).Interface().(runtime.Object), nil
}

func (r *GenericResourceReconciler) CreateIfNotExist(desired runtime.Object, desiredState DesiredState) (bool, runtime.Object, error) {
	current, err := r.fromDesired(desired)
	if err != nil {
		return false, nil, errors.WrapIf(err, "failed to create new object based on desired")
	}
	m, err := meta.Accessor(desired)
	if err != nil {
		return false, nil, errors.WrapIf(err, "failed to get object key")
	}
	key := client.ObjectKey{Namespace: m.GetNamespace(), Name: m.GetName()}
	resourceDetails, _, err := r.resourceDetails(desired)
	if err != nil {
		return false, nil, errors.WrapIf(err, "failed to get resource details")
	}
	log := r.resourceLog(desired, resourceDetails...)
	traceLog := log.V(2)
	err = r.Client.Get(context.TODO(), key, current.(client.Object))
	current.GetObjectKind().SetGroupVersionKind(desired.GetObjectKind().GroupVersionKind())
	if err != nil && !apierrors.IsNotFound(err) {
		return false, nil, errors.WrapIfWithDetails(err, "getting resource failed", resourceDetails...)
	}
	if apierrors.IsNotFound(err) {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(desired); err != nil {
			log.Error(err, "Failed to set last applied annotation", "desired", desired)
		}
		if desiredState != nil {
			err = desiredState.BeforeCreate(desired)
			if err != nil {
				return false, nil, errors.WrapIfWithDetails(err, "failed to prepare desired state before creation", resourceDetails...)
			}
			if ds, ok := desiredState.(DesiredStateShouldCreate); ok {
				should, err := ds.ShouldCreate(desired)
				if err != nil {
					return false, desired, err
				}
				if !should {
					return false, desired, nil
				}
			}
		}
		var createOptions []client.CreateOption
		if ds, ok := desiredState.(DesiredStateWithCreateOptions); ok {
			createOptions = append(createOptions, ds.GetCreateOptions()...)
		}
		if err := r.Client.Create(context.TODO(), desired.(client.Object), createOptions...); err != nil {
			return false, nil, errors.WrapIfWithDetails(err, "creating resource failed", resourceDetails...)
		}
		switch t := desired.DeepCopyObject().(type) {
		case *v1beta1.CustomResourceDefinition:
			err = wait.PollUntilContextTimeout(context.TODO(), time.Second*1, time.Second*10, false, func(ctx context.Context) (done bool, err error) {
				err = r.Client.Get(ctx, client.ObjectKey{Namespace: t.Namespace, Name: t.Name}, t)
				if err != nil {
					return false, err
				}
				return crdReady(t), nil
			})
			if err != nil {
				return false, nil, errors.WrapIfWithDetails(err, "failed to wait for the crd to get ready", resourceDetails...)
			}
		case *v1.CustomResourceDefinition:
			err = wait.PollUntilContextTimeout(context.TODO(), time.Second*1, time.Second*10, false, func(ctx context.Context) (done bool, err error) {
				err = r.Client.Get(ctx, client.ObjectKey{Namespace: t.Namespace, Name: t.Name}, t)
				if err != nil {
					return false, err
				}
				return crdReadyV1(t), nil
			})
			if err != nil {
				return false, nil, errors.WrapIfWithDetails(err, "failed to wait for the crd to get ready", resourceDetails...)
			}
		}
		log.Info("resource created")
		return true, current, nil
	}
	traceLog.Info("resource already exists")
	return false, current, nil
}

func (r *GenericResourceReconciler) delete(desired runtime.Object, desiredState DesiredState) (bool, error) {
	current, err := r.fromDesired(desired)
	if err != nil {
		return false, errors.WrapIf(err, "failed to create new object based on desired")
	}
	m, err := meta.Accessor(desired)
	if err != nil {
		return false, errors.WrapIf(err, "failed to get object key")
	}
	key := client.ObjectKey{Namespace: m.GetNamespace(), Name: m.GetName()}
	resourceDetails, _, err := r.resourceDetails(desired)
	if err != nil {
		return false, errors.WrapIf(err, "failed to get resource details")
	}
	log := r.resourceLog(desired, resourceDetails...)
	debugLog := log.V(1)
	traceLog := log.V(2)
	err = r.Client.Get(context.TODO(), key, current.(client.Object))
	if err != nil {
		// If the resource type does not exist we should be ok to move on
		if meta.IsNoMatchError(err) || runtime.IsNotRegisteredError(err) {
			return false, nil
		}
		if !apierrors.IsNotFound(err) {
			return false, errors.WrapIfWithDetails(err, "getting resource failed", resourceDetails...)
		} else {
			traceLog.Info("resource not found skipping delete")
			return false, nil
		}
	}
	if desiredState != nil {
		err = desiredState.BeforeDelete(current)
		if err != nil {
			return false, errors.WrapIfWithDetails(err, "failed to prepare desired state before deletion", resourceDetails...)
		}
		if ds, ok := desiredState.(DesiredStateShouldDelete); ok {
			should, err := ds.ShouldDelete(desired)
			if err != nil {
				return false, err
			}
			if !should {
				return false, nil
			}
		}
	}
	var deleteOptions []client.DeleteOption
	if ds, ok := desiredState.(DesiredStateWithDeleteOptions); ok {
		deleteOptions = append(deleteOptions, ds.GetDeleteOptions()...)
	}
	err = r.Client.Delete(context.TODO(), current.(client.Object), deleteOptions...)
	if err != nil {
		return false, errors.WrapIfWithDetails(err, "failed to delete resource", resourceDetails...)
	}
	debugLog.Info("resource deleted")
	return true, nil
}

func crdReady(crd *v1beta1.CustomResourceDefinition) bool {
	for _, cond := range crd.Status.Conditions {
		switch cond.Type {
		case v1beta1.Established:
			if cond.Status == v1beta1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

func crdReadyV1(crd *v1.CustomResourceDefinition) bool {
	for _, cond := range crd.Status.Conditions {
		switch cond.Type {
		case v1.Established:
			if cond.Status == v1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

func (r *GenericResourceReconciler) resourceDetails(desired runtime.Object) ([]interface{}, schema.GroupVersionKind, error) {
	gvk := schema.GroupVersionKind{}
	m, err := meta.Accessor(desired)
	if err != nil {
		return nil, gvk, errors.WithStackIf(err)
	}
	key := client.ObjectKey{Namespace: m.GetNamespace(), Name: m.GetName()}
	values := []interface{}{"name", key.Name}
	if key.Namespace != "" {
		values = append(values, "namespace", key.Namespace)
	}
	defaultValues := append(values, "type", reflect.TypeOf(desired).String())
	if r.Options.Scheme == nil {
		return defaultValues, gvk, nil
	}
	gvk, err = apiutil.GVKForObject(desired, r.Options.Scheme)
	if err != nil {
		r.Log.V(2).Info("unable to get gvk for resource, falling back to type")
		return values, gvk, nil
	}
	values = append(values,
		"apiVersion", gvk.GroupVersion().String(),
		"kind", gvk.Kind)
	return values, gvk, nil
}

func (r *GenericResourceReconciler) resourceLog(desired runtime.Object, details ...interface{}) logr.Logger {
	if len(details) > 0 {
		return r.Log.WithValues(details...)
	}
	return r.Log
}
