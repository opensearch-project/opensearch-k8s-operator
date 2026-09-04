package controllers

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
)

// Controllers are safe for MaxConcurrentReconciles > 1 only while no
// per-request state lives on the shared reconciler struct, so guard the
// struct shapes.
func TestReconcilerStructsHaveNoPerRequestState(t *testing.T) {
	forbidden := []reflect.Type{
		reflect.TypeOf((*opensearchv1.OpenSearchCluster)(nil)),
		reflect.TypeOf((*opensearchv1.OpensearchUser)(nil)),
		reflect.TypeOf((*opensearchv1.OpensearchRole)(nil)),
		reflect.TypeOf((*opensearchv1.OpensearchTenant)(nil)),
		reflect.TypeOf((*opensearchv1.OpensearchUserRoleBinding)(nil)),
		reflect.TypeOf((*opensearchv1.OpensearchActionGroup)(nil)),
		reflect.TypeOf((*opensearchv1.OpenSearchISMPolicy)(nil)),
		reflect.TypeOf((*opensearchv1.OpensearchIndexTemplate)(nil)),
		reflect.TypeOf((*opensearchv1.OpensearchComponentTemplate)(nil)),
		reflect.TypeOf((*opensearchv1.OpensearchSnapshotPolicy)(nil)),
		reflect.TypeOf(logr.Logger{}),
	}

	structs := []any{
		OpenSearchClusterReconciler{},
		OpensearchUserReconciler{},
		OpensearchRoleReconciler{},
		OpensearchTenantReconciler{},
		OpensearchUserRoleBindingReconciler{},
		OpensearchActionGroupReconciler{},
		OpensearchISMPolicyReconciler{},
		OpensearchIndexTemplateReconciler{},
		OpensearchComponentTemplateReconciler{},
		OpensearchSnapshotPolicyReconciler{},
	}

	for _, s := range structs {
		rt := reflect.TypeOf(s)
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			for _, ft := range forbidden {
				if f.Type == ft {
					t.Errorf("%s.%s has type %s: per-request state on the shared reconciler is not safe with MaxConcurrentReconciles > 1", rt.Name(), f.Name, ft)
				}
			}
		}
	}
}

// Reconcile many distinct clusters at once through one reconciler and check
// that every cluster ends up with its own finalizer and phase. With per-request
// state on the shared struct, in-flight reconciles overwrite each other's
// instance and some clusters never get updated (and `go test -race` flags it).
func TestClusterReconcilerConcurrentReconcilesDoNotShareState(t *testing.T) {
	const clusters = 32
	scheme := runtime.NewScheme()
	if err := opensearchv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	objs := make([]client.Object, 0, clusters)
	for i := 0; i < clusters; i++ {
		objs = append(objs, &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("cluster-%d", i), Namespace: "default"},
		})
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&opensearchv1.OpenSearchCluster{}).
		WithObjects(objs...).
		Build()
	r := &OpenSearchClusterReconciler{Client: c, Scheme: scheme, Recorder: record.NewFakeRecorder(clusters)}

	var wg sync.WaitGroup
	errs := make(chan error, clusters)
	for i := 0; i < clusters; i++ {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: "default"}}
			if _, err := r.Reconcile(context.Background(), req); err != nil {
				errs <- fmt.Errorf("%s: %w", name, err)
			}
		}(objs[i].GetName())
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	for _, o := range objs {
		got := &opensearchv1.OpenSearchCluster{}
		if err := c.Get(context.Background(), client.ObjectKeyFromObject(o), got); err != nil {
			t.Fatal(err)
		}
		if !controllerutil.ContainsFinalizer(got, "Opensearch") {
			t.Errorf("%s: finalizer missing", got.Name)
		}
		if got.Status.Phase != opensearchv1.PhaseRunning {
			t.Errorf("%s: phase = %q, want %q", got.Name, got.Status.Phase, opensearchv1.PhaseRunning)
		}
	}
}
