package controllers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// These controllers requeue their objects every 30s, so a reconcile that finds
// the finalizer already in place must not send an update for it: with hundreds
// of objects the no-op updates keep the operator and the API server busy.
var _ = Describe("Finalizer updates of the opensearch.org resource controllers", func() {
	type reconcilerFunc func(c client.Client, scheme *runtime.Scheme) interface {
		Reconcile(context.Context, ctrl.Request) (ctrl.Result, error)
	}
	type controllerCase struct {
		kind      string
		object    func() client.Object
		reconcile reconcilerFunc
	}
	recorder := func() record.EventRecorder { return record.NewFakeRecorder(100) }
	cases := []controllerCase{
		{"OpensearchUser", func() client.Object { return &opensearchv1.OpensearchUser{} }, func(c client.Client, s *runtime.Scheme) interface {
			Reconcile(context.Context, ctrl.Request) (ctrl.Result, error)
		} {
			return &OpensearchUserReconciler{Client: c, Scheme: s, Recorder: recorder()}
		}},
		{"OpensearchRole", func() client.Object { return &opensearchv1.OpensearchRole{} }, func(c client.Client, s *runtime.Scheme) interface {
			Reconcile(context.Context, ctrl.Request) (ctrl.Result, error)
		} {
			return &OpensearchRoleReconciler{Client: c, Scheme: s, Recorder: recorder()}
		}},
		{"OpensearchUserRoleBinding", func() client.Object { return &opensearchv1.OpensearchUserRoleBinding{} }, func(c client.Client, s *runtime.Scheme) interface {
			Reconcile(context.Context, ctrl.Request) (ctrl.Result, error)
		} {
			return &OpensearchUserRoleBindingReconciler{Client: c, Scheme: s, Recorder: recorder()}
		}},
		{"OpensearchTenant", func() client.Object { return &opensearchv1.OpensearchTenant{} }, func(c client.Client, s *runtime.Scheme) interface {
			Reconcile(context.Context, ctrl.Request) (ctrl.Result, error)
		} {
			return &OpensearchTenantReconciler{Client: c, Scheme: s, Recorder: recorder()}
		}},
		{"OpensearchActionGroup", func() client.Object { return &opensearchv1.OpensearchActionGroup{} }, func(c client.Client, s *runtime.Scheme) interface {
			Reconcile(context.Context, ctrl.Request) (ctrl.Result, error)
		} {
			return &OpensearchActionGroupReconciler{Client: c, Scheme: s, Recorder: recorder()}
		}},
		{"OpensearchIndexTemplate", func() client.Object { return &opensearchv1.OpensearchIndexTemplate{} }, func(c client.Client, s *runtime.Scheme) interface {
			Reconcile(context.Context, ctrl.Request) (ctrl.Result, error)
		} {
			return &OpensearchIndexTemplateReconciler{Client: c, Scheme: s, Recorder: recorder()}
		}},
		{"OpensearchComponentTemplate", func() client.Object { return &opensearchv1.OpensearchComponentTemplate{} }, func(c client.Client, s *runtime.Scheme) interface {
			Reconcile(context.Context, ctrl.Request) (ctrl.Result, error)
		} {
			return &OpensearchComponentTemplateReconciler{Client: c, Scheme: s, Recorder: recorder()}
		}},
		{"OpenSearchISMPolicy", func() client.Object { return &opensearchv1.OpenSearchISMPolicy{} }, func(c client.Client, s *runtime.Scheme) interface {
			Reconcile(context.Context, ctrl.Request) (ctrl.Result, error)
		} {
			return &OpensearchISMPolicyReconciler{Client: c, Scheme: s, Recorder: recorder()}
		}},
		{"OpensearchSnapshotPolicy", func() client.Object { return &opensearchv1.OpensearchSnapshotPolicy{} }, func(c client.Client, s *runtime.Scheme) interface {
			Reconcile(context.Context, ctrl.Request) (ctrl.Result, error)
		} {
			return &OpensearchSnapshotPolicyReconciler{Client: c, Scheme: s, Recorder: recorder()}
		}},
	}

	// reconcileOnce reconciles obj and returns how many full updates the
	// controller sent for it, and the object as stored afterwards.
	reconcileOnce := func(tc controllerCase, finalizers []string) (int, client.Object) {
		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(opensearchv1.AddToScheme(scheme)).To(Succeed())

		obj := tc.object()
		obj.SetName("finalizer-update")
		obj.SetNamespace("default")
		obj.SetFinalizers(finalizers)

		updates := 0
		c := interceptor.NewClient(
			fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).WithStatusSubresource(obj).Build(),
			interceptor.Funcs{
				Update: func(ctx context.Context, c client.WithWatch, o client.Object, opts ...client.UpdateOption) error {
					updates++
					return c.Update(ctx, o, opts...)
				},
			},
		)

		_, _ = tc.reconcile(c, scheme).Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(obj)})

		stored := tc.object()
		Expect(c.Get(context.Background(), client.ObjectKeyFromObject(obj), stored)).To(Succeed())
		return updates, stored
	}

	for _, tc := range cases {
		It("does not update a "+tc.kind+" that already has the finalizer", func() {
			updates, stored := reconcileOnce(tc, []string{OpensearchFinalizer})
			Expect(updates).To(BeZero())
			Expect(controllerutil.ContainsFinalizer(stored, OpensearchFinalizer)).To(BeTrue())
		})

		It("adds the finalizer to a new "+tc.kind+" with one update", func() {
			updates, stored := reconcileOnce(tc, nil)
			Expect(updates).To(Equal(1))
			Expect(controllerutil.ContainsFinalizer(stored, OpensearchFinalizer)).To(BeTrue())
		})
	}
})
