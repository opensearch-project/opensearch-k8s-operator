package reconcilers

import (
	"context"
	"time"

	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	k8smock "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	ot "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconciler"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("StatusConfigMapReconciler", func() {
	var (
		transport  *httpmock.MockTransport
		reconciler *StatusConfigMapReconciler
		instance   *opensearchv1.OpenSearchCluster
		recorder   *record.FakeRecorder
		mockClient *k8smock.MockK8sClient
	)

	BeforeEach(func() {
		mockClient = k8smock.NewMockK8sClient(GinkgoT())
		transport = httpmock.NewMockTransport()
		transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
		recorder = record.NewFakeRecorder(10)

		instance = &opensearchv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
			},
			Spec: opensearchv1.ClusterSpec{
				General: opensearchv1.GeneralConfig{
					ServiceName: "test-cluster",
					HttpPort:    9200,
					StatusConfigMap: opensearchv1.StatusConfigMapConfig{
						Enable: true,
					},
				},
				NodePools: []opensearchv1.NodePool{
					{Component: "node", Roles: []string{"master", "data"}},
				},
			},
			Status: opensearchv1.ClusterStatus{Phase: opensearchv1.PhaseRunning},
		}
	})

	JustBeforeEach(func() {
		options := ReconcilerOptions{}
		options.apply(WithOSClientTransport(transport), WithUpdateStatus(false))
		reconciler = &StatusConfigMapReconciler{
			client:            mockClient,
			ctx:               context.Background(),
			ReconcilerOptions: options,
			recorder:          recorder,
			instance:          instance,
			logger:            log.FromContext(context.Background()),
		}
	})

	When("the feature is disabled", func() {
		BeforeEach(func() {
			instance.Spec.General.StatusConfigMap.Enable = false
		})

		It("deletes the ConfigMap and does not requeue", func() {
			mockClient.EXPECT().
				ReconcileResource(
					mock.MatchedBy(func(obj runtime.Object) bool {
						cm, ok := obj.(*corev1.ConfigMap)
						return ok && cm.Name == "test-cluster-status"
					}),
					ot.StateAbsent,
				).
				Return(nil, nil).Once()

			result, err := reconciler.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
		})
	})

	When("the ConfigMap annotation shows the interval has NOT elapsed", func() {
		BeforeEach(func() {
			// Written 10 s ago; default interval is 60 s → skip.
			recent := time.Now().UTC().Add(-10 * time.Second).Format(time.RFC3339)
			mockClient.EXPECT().
				GetConfigMap("test-cluster-status", "default").
				Return(corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							statusConfigMapLastUpdateAnnotation: recent,
						},
					},
				}, nil).Once()
		})

		It("skips the expensive API calls and does not self-requeue", func() {
			result, err := reconciler.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
		})
	})

	When("updateInterval is set and that interval has NOT elapsed", func() {
		BeforeEach(func() {
			d := metav1.Duration{Duration: 10 * time.Minute}
			instance.Spec.General.StatusConfigMap.UpdateInterval = &d
			// Written 2 minutes ago — still within 10 m.
			recent := time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339)
			mockClient.EXPECT().
				GetConfigMap("test-cluster-status", "default").
				Return(corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							statusConfigMapLastUpdateAnnotation: recent,
						},
					},
				}, nil).Once()
		})

		It("respects the custom interval and skips", func() {
			result, err := reconciler.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
		})
	})

	When("the ConfigMap annotation shows the interval HAS elapsed", func() {
		BeforeEach(func() {
			// Written 90 s ago; default interval is 60 s → refresh.
			old := time.Now().UTC().Add(-90 * time.Second).Format(time.RFC3339)
			mockClient.EXPECT().
				GetConfigMap("test-cluster-status", "default").
				Return(corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							statusConfigMapLastUpdateAnnotation: old,
						},
					},
				}, nil).Once()

			// Use a silent 404 no-responder so unregistered _cat/* paths don't call Fail().
			transport.RegisterNoResponder(httpmock.NewStringResponder(404, "not found"))
			registerOsPingResponders(transport, instance)

			mockClient.EXPECT().
				GetSecret("test-cluster-admin-password", "default").
				Return(corev1.Secret{
					Data: map[string][]byte{
						"username": []byte("admin"),
						"password": []byte("admin"),
					},
				}, nil).Once()
			mockClient.EXPECT().Scheme().Return(scheme.Scheme).Once()
			mockClient.EXPECT().
				CreateConfigMap(mock.MatchedBy(func(cm *corev1.ConfigMap) bool {
					return cm.Name == "test-cluster-status" &&
						cm.Annotations[statusConfigMapLastUpdateAnnotation] != ""
				})).
				Return(nil, nil).Once()
		})

		It("writes the ConfigMap and does not self-requeue", func() {
			result, err := reconciler.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
		})
	})

	When("the ConfigMap does not exist yet (first run)", func() {
		BeforeEach(func() {
			mockClient.EXPECT().
				GetConfigMap("test-cluster-status", "default").
				Return(corev1.ConfigMap{}, NotFoundError()).Once()

			// Use a silent 404 no-responder so unregistered _cat/* paths don't call Fail().
			transport.RegisterNoResponder(httpmock.NewStringResponder(404, "not found"))
			registerOsPingResponders(transport, instance)

			mockClient.EXPECT().
				GetSecret("test-cluster-admin-password", "default").
				Return(corev1.Secret{
					Data: map[string][]byte{
						"username": []byte("admin"),
						"password": []byte("admin"),
					},
				}, nil).Once()
			mockClient.EXPECT().Scheme().Return(scheme.Scheme).Once()
			mockClient.EXPECT().
				CreateConfigMap(mock.MatchedBy(func(cm *corev1.ConfigMap) bool {
					return cm.Name == "test-cluster-status"
				})).
				Return(nil, nil).Once()
		})

		It("creates the ConfigMap without self-requeueing", func() {
			result, err := reconciler.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
		})
	})
})
