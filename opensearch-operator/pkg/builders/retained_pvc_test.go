package builders

import (
	"context"
	"errors"
	"testing"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// getErrorClient returns the injected error from Get instead of consulting the fake store.
type getErrorClient struct {
	client.Client
	err error
}

func (c *getErrorClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.err
}

func retainedPVCTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := opensearchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add opensearch scheme: %v", err)
	}
	return scheme
}

func retainedPVCCluster(name, ns string, pools []opensearchv1.NodePool) *opensearchv1.OpenSearchCluster {
	return &opensearchv1.OpenSearchCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: opensearchv1.ClusterSpec{
			General:   opensearchv1.GeneralConfig{ServiceName: name, Version: "2.11.0"},
			NodePools: pools,
		},
	}
}

func retainedMasterPool(component string, persistence *opensearchv1.PersistenceConfig) opensearchv1.NodePool {
	return opensearchv1.NodePool{
		Component:   component,
		Replicas:    1,
		Roles:       []string{"cluster_manager", "data"},
		Persistence: persistence,
	}
}

func retainedDataOnlyPool(component string) opensearchv1.NodePool {
	return opensearchv1.NodePool{
		Component: component,
		Replicas:  1,
		Roles:     []string{"data"},
	}
}

func retainedDataPVC(cr *opensearchv1.OpenSearchCluster, pool *opensearchv1.NodePool) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-" + StsName(cr, pool) + "-0",
			Namespace: cr.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
			},
		},
	}
}

func TestHasRetainedMasterPVC(t *testing.T) {
	scheme := retainedPVCTestScheme(t)

	t.Run("false when no master data PVC exists", func(t *testing.T) {
		cr := retainedPVCCluster("fresh", "ns", []opensearchv1.NodePool{retainedMasterPool("masters", nil)})
		c := fake.NewClientBuilder().WithScheme(scheme).Build()

		found, err := HasRetainedMasterPVC(context.Background(), c, cr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Fatal("expected found=false")
		}
	})

	t.Run("true when ordinal 0 of a PVC-backed master pool exists", func(t *testing.T) {
		pool := retainedMasterPool("masters", nil)
		cr := retainedPVCCluster("retained", "ns", []opensearchv1.NodePool{pool})
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(retainedDataPVC(cr, &pool)).Build()

		found, err := HasRetainedMasterPVC(context.Background(), c, cr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("expected found=true")
		}
	})

	t.Run("skips emptyDir master pools", func(t *testing.T) {
		pool := retainedMasterPool("masters", &opensearchv1.PersistenceConfig{
			PersistenceSource: opensearchv1.PersistenceSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		})
		cr := retainedPVCCluster("emptydir", "ns", []opensearchv1.NodePool{pool})
		// Even with a same-named PVC present, emptyDir pools are not PVC-backed.
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(retainedDataPVC(cr, &pool)).Build()

		found, err := HasRetainedMasterPVC(context.Background(), c, cr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Fatal("expected found=false for emptyDir pool")
		}
	})

	t.Run("ignores data-only pool PVCs", func(t *testing.T) {
		data := retainedDataOnlyPool("data")
		cr := retainedPVCCluster("dataonly", "ns", []opensearchv1.NodePool{
			retainedMasterPool("masters", nil),
			data,
		})
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(retainedDataPVC(cr, &data)).Build()

		found, err := HasRetainedMasterPVC(context.Background(), c, cr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Fatal("expected found=false when only a data-pool PVC exists")
		}
	})

	t.Run("returns Get error when it is not NotFound", func(t *testing.T) {
		cr := retainedPVCCluster("err", "ns", []opensearchv1.NodePool{retainedMasterPool("masters", nil)})
		injected := errors.New("apiserver unavailable")
		c := &getErrorClient{Client: fake.NewClientBuilder().WithScheme(scheme).Build(), err: injected}

		found, err := HasRetainedMasterPVC(context.Background(), c, cr)
		if found {
			t.Fatal("expected found=false on error")
		}
		if !errors.Is(err, injected) {
			t.Fatalf("expected injected error, got %v", err)
		}
	})
}
