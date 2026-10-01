package controllers

import (
	"context"
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Regression test for #1496: adding and removing the operator's finalizer must
// not make the operator's field manager own spec fields, otherwise a later
// server-side apply from Helm 4 fails with a conflict.
var _ = Describe("Cluster finalizer and server-side apply", Ordered, func() {
	const (
		clusterName = "finalizer-ssa-test"
		namespace   = clusterName
		finalizer   = "Opensearch"
		timeout     = time.Second * 30
		interval    = time.Second * 1
	)
	var (
		ctx = context.Background()
		key = client.ObjectKey{Name: clusterName, Namespace: namespace}
		// helmApply mimics `helm upgrade`: server-side apply as "helm", no force.
		helmApply = func(replicas int64) error {
			u := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "opensearch.org/v1",
				"kind":       "OpenSearchCluster",
				"metadata":   map[string]any{"name": clusterName, "namespace": namespace},
				"spec": map[string]any{
					"general": map[string]any{"serviceName": clusterName, "version": "2.0.0"},
					"nodePools": []any{map[string]any{
						"component": "masters",
						"replicas":  replicas,
						"diskSize":  "5Gi",
						"roles":     []any{"master", "data"},
					}},
				},
			}}
			return k8sClient.Patch(ctx, u, client.Apply, client.FieldOwner("helm"))
		}
		get = func() *opensearchv1.OpenSearchCluster {
			c := &opensearchv1.OpenSearchCluster{}
			Expect(k8sClient.Get(ctx, key, c)).To(Succeed())
			return c
		}
	)

	It("should create the namespace", func() {
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).To(Succeed())
	})

	It("should let the operator add its finalizer to an applied cluster", func() {
		Expect(helmApply(3)).To(Succeed())
		// The client reads from an informer cache, so the object can lag behind the apply.
		Eventually(func() []string {
			c := &opensearchv1.OpenSearchCluster{}
			if err := k8sClient.Get(ctx, key, c); err != nil {
				return nil
			}
			return c.Finalizers
		}, timeout, interval).Should(ContainElement(finalizer))
	})

	It("should leave the operator owning nothing but the finalizer", func() {
		for _, mf := range get().ManagedFields {
			if mf.Manager == "helm" || mf.Subresource != "" {
				continue
			}
			fields := map[string]map[string]any{}
			Expect(json.Unmarshal(mf.FieldsV1.Raw, &fields)).To(Succeed(), "manager %s", mf.Manager)
			Expect(fields).To(HaveLen(1), "manager %s owns %s", mf.Manager, mf.FieldsV1.Raw)
			Expect(fields).To(HaveKey("f:metadata"), "manager %s owns %s", mf.Manager, mf.FieldsV1.Raw)
			Expect(fields["f:metadata"]).To(And(HaveLen(1), HaveKey("f:finalizers")), "manager %s owns %s", mf.Manager, mf.FieldsV1.Raw)
		}
	})

	It("should let Helm apply again without forcing conflicts", func() {
		Expect(helmApply(5)).To(Succeed())
		Eventually(func() int32 { return get().Spec.NodePools[0].Replicas }, timeout, interval).Should(BeEquivalentTo(5))
		Expect(get().Finalizers).To(ContainElement(finalizer))
	})

	It("should remove the finalizer on deletion", func() {
		Expect(k8sClient.Delete(ctx, get())).To(Succeed())
		Eventually(func() bool {
			return errors.IsNotFound(k8sClient.Get(ctx, key, &opensearchv1.OpenSearchCluster{}))
		}, timeout, interval).Should(BeTrue())
	})
})

// The migration controller writes its own finalizer to the same list, and a
// merge patch replaces the whole list, so a stale patch must conflict instead
// of dropping the other controller's entry.
var _ = Describe("patchMetadata", func() {
	It("should conflict instead of overwriting finalizers added concurrently", func() {
		ctx := context.Background()
		scheme := runtime.NewScheme()
		Expect(opensearchv1.AddToScheme(scheme)).To(Succeed())
		cluster := &opensearchv1.OpenSearchCluster{ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns"}}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()

		stale := &opensearchv1.OpenSearchCluster{}
		Expect(c.Get(ctx, client.ObjectKeyFromObject(cluster), stale)).To(Succeed())
		other := stale.DeepCopy()
		Expect(patchMetadata(ctx, c, other, func() { other.Finalizers = append(other.Finalizers, "Migration") })).To(Succeed())

		err := patchMetadata(ctx, c, stale, func() { stale.Finalizers = append(stale.Finalizers, "Opensearch") })
		Expect(errors.IsConflict(err)).To(BeTrue(), "got %v", err)

		fresh := &opensearchv1.OpenSearchCluster{}
		Expect(c.Get(ctx, client.ObjectKeyFromObject(cluster), fresh)).To(Succeed())
		Expect(fresh.Finalizers).To(Equal([]string{"Migration"}))
	})
})
