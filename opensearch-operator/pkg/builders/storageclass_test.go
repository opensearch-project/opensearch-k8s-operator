package builders

import (
	"testing"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStorageClassHandling(t *testing.T) {
	cr := &opsterv1.OpenSearchCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
				HttpPort:    9200,
				ServiceName: "test-service",
				Version:     "2.2.1",
			},
		},
	}

	t.Run("should return nil when persistence is nil", func(t *testing.T) {
		nodePool := opsterv1.NodePool{
			Component:   "masters",
			Replicas:    3,
			DiskSize:    "1Gi",
			Roles:       []string{"cluster_manager", "data"},
			Persistence: nil, // No persistence specified
		}

		sts := NewSTSForNodePool("test", cr, nodePool, "checksum", nil, nil, nil)
		
		if len(sts.Spec.VolumeClaimTemplates) == 0 {
			t.Fatal("Expected VolumeClaimTemplate to be created when persistence is nil")
		}
		
		actual := sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName
		if actual != nil {
			t.Errorf("Expected storageClassName to be nil when persistence is nil, got %v", actual)
		}
	})

	t.Run("should return nil when storageClassName is not specified in PVC", func(t *testing.T) {
		nodePool := opsterv1.NodePool{
			Component: "masters",
			Replicas:  3,
			DiskSize:  "1Gi",
			Roles:     []string{"cluster_manager", "data"},
			Persistence: &opsterv1.PersistenceConfig{
				PersistenceSource: opsterv1.PersistenceSource{
					PVC: &opsterv1.PVCSource{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						// StorageClassName not specified (nil)
					},
				},
			},
		}

		sts := NewSTSForNodePool("test", cr, nodePool, "checksum", nil, nil, nil)
		
		if len(sts.Spec.VolumeClaimTemplates) == 0 {
			t.Fatal("Expected VolumeClaimTemplate to be created")
		}
		
		actual := sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName
		if actual != nil {
			t.Errorf("Expected storageClassName to be nil when not specified, got %v", actual)
		}
	})

	t.Run("should return pointer to empty string when explicitly set to empty", func(t *testing.T) {
		emptyString := ""
		nodePool := opsterv1.NodePool{
			Component: "masters",
			Replicas:  3,
			DiskSize:  "1Gi",
			Roles:     []string{"cluster_manager", "data"},
			Persistence: &opsterv1.PersistenceConfig{
				PersistenceSource: opsterv1.PersistenceSource{
					PVC: &opsterv1.PVCSource{
						StorageClassName: &emptyString,
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					},
				},
			},
		}

		sts := NewSTSForNodePool("test", cr, nodePool, "checksum", nil, nil, nil)
		
		if len(sts.Spec.VolumeClaimTemplates) == 0 {
			t.Fatal("Expected VolumeClaimTemplate to be created")
		}
		
		actual := sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName
		if actual == nil {
			t.Error("Expected storageClassName to be pointer to empty string, got nil")
		} else if *actual != "" {
			t.Errorf("Expected storageClassName to be empty string, got %v", *actual)
		}
	})

	t.Run("should return pointer to specific class when specified", func(t *testing.T) {
		specificClass := "fast-ssd"
		nodePool := opsterv1.NodePool{
			Component: "masters",
			Replicas:  3,
			DiskSize:  "1Gi",
			Roles:     []string{"cluster_manager", "data"},
			Persistence: &opsterv1.PersistenceConfig{
				PersistenceSource: opsterv1.PersistenceSource{
					PVC: &opsterv1.PVCSource{
						StorageClassName: &specificClass,
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					},
				},
			},
		}

		sts := NewSTSForNodePool("test", cr, nodePool, "checksum", nil, nil, nil)
		
		if len(sts.Spec.VolumeClaimTemplates) == 0 {
			t.Fatal("Expected VolumeClaimTemplate to be created")
		}
		
		actual := sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName
		if actual == nil {
			t.Error("Expected storageClassName to be pointer to specific class, got nil")
		} else if *actual != specificClass {
			t.Errorf("Expected storageClassName to be %s, got %v", specificClass, *actual)
		}
	})
}