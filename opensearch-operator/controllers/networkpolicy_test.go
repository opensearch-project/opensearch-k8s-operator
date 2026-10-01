package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

type networkPolicyWatchClient struct {
	client.Client
	requests chan client.ObjectKey
}

func (c *networkPolicyWatchClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	err := c.Client.Get(ctx, key, obj, opts...)
	if _, ok := obj.(*opensearchv1.OpenSearchCluster); ok {
		select {
		case c.requests <- key:
		case <-ctx.Done():
		}
	}
	return err
}

var _ = Describe("NetworkPolicy watch", func() {
	It("should enqueue the owning cluster when a policy is edited or deleted", func() {
		ctx, stop := context.WithCancel(context.Background())
		defer stop()
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "networkpolicy-watch-"}}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(context.Background(), ns)).To(Succeed()) })

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:     scheme.Scheme,
			Metrics:    metricsserver.Options{BindAddress: "0"},
			Cache:      cache.Options{DefaultNamespaces: map[string]cache.Config{ns.Name: {}}},
			Controller: config.Controller{SkipNameValidation: ptr.To(true)},
		})
		Expect(err).NotTo(HaveOccurred())
		requests := make(chan client.ObjectKey, 10)
		r := &OpenSearchClusterReconciler{
			Client: &networkPolicyWatchClient{Client: mgr.GetClient(), requests: requests},
			Scheme: scheme.Scheme,
		}
		Expect(r.SetupWithManager(mgr)).To(Succeed())
		done := make(chan error, 1)
		go func() { done <- mgr.Start(ctx) }()
		DeferCleanup(func() {
			stop()
			Eventually(done, 10*time.Second).Should(Receive(Succeed()))
		})
		Expect(mgr.GetCache().WaitForCacheSync(ctx)).To(BeTrue())

		// A missing owner makes Reconcile return without a periodic requeue or
		// child-resource updates, so only the policy watch can trigger requests.
		owner := client.ObjectKey{Namespace: ns.Name, Name: "missing-cluster"}
		policy := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: owner.Name + "-network-policy", Namespace: ns.Name,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: opensearchv1.GroupVersion.String(), Kind: "OpenSearchCluster",
					Name: owner.Name, UID: "missing-cluster", Controller: ptr.To(true),
				}},
			},
			Spec: networkingv1.NetworkPolicySpec{PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}},
		}
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())
		Eventually(requests, 10*time.Second).Should(Receive(Equal(owner)))
		Consistently(requests, 200*time.Millisecond).ShouldNot(Receive())

		policy.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{{}}
		Expect(k8sClient.Update(ctx, policy)).To(Succeed())
		Eventually(requests, 10*time.Second).Should(Receive(Equal(owner)))
		Consistently(requests, 200*time.Millisecond).ShouldNot(Receive())

		Expect(k8sClient.Delete(ctx, policy)).To(Succeed())
		Eventually(requests, 10*time.Second).Should(Receive(Equal(owner)))
	})
})
