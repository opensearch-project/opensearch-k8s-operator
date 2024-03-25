package reconcilers

import (
	"context"
	"fmt"
	"net/http"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/mocks/github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("snapshot repositories reconciler", func() {
	var (
		transport  *httpmock.MockTransport
		reconciler *SnapshotRepositoryReconciler
		instance   *opsterv1.OpenSearchCluster
		recorder   *record.FakeRecorder
		mockClient *k8s.MockK8sClient
	)
	const (
		repoName = "testrepo"
	)

	BeforeEach(func() {
		mockClient = k8s.NewMockK8sClient(GinkgoT())
		transport = httpmock.NewMockTransport()
		transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
		instance = &opsterv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-snapshotrepo",
				Namespace: "test",
			},
			Spec: opsterv1.ClusterSpec{
				General: opsterv1.GeneralConfig{
					ServiceName: "test-snapshotrepo",
					HttpPort:    9200,
					SnapshotRepositories: []opsterv1.SnapshotRepoConfig{
						{
							Name: repoName,
							Type: "fs",
							Settings: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
				NodePools: []opsterv1.NodePool{
					{
						Component: "node",
						Roles: []string{
							"master",
							"data",
						},
					},
				},
			},
			Status: opsterv1.ClusterStatus{
				Phase: opsterv1.PhasePending,
			},
		}
	})

	JustBeforeEach(func() {
		options := ReconcilerOptions{}
		options.apply(WithOSClientTransport(transport), WithUpdateStatus(false))
		reconciler = &SnapshotRepositoryReconciler{
			client:            mockClient,
			ctx:               context.Background(),
			ReconcilerOptions: options,
			recorder:          recorder,
			instance:          instance,
			logger:            log.FromContext(context.Background()),
		}
	})

	When("cluster is not ready", func() {
		BeforeEach(func() {
			instance.Status.Phase = opsterv1.PhasePending
			recorder = record.NewFakeRecorder(1)
		})
		It("should wait for the cluster to be running", func() {
			go func() {
				defer GinkgoRecover()
				defer close(recorder.Events)
				result, err := reconciler.Reconcile()
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeTrue())
			}()
			var events []string
			for msg := range recorder.Events {
				events = append(events, msg)
			}
			Expect(len(events)).To(Equal(1))
			Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s waiting for opensearch cluster status to be running", opensearchPending)))
		})
	})

	Context("cluster is ready", func() {
		extraContextCalls := 1
		BeforeEach(func() {
			instance.Status.Phase = opsterv1.PhaseRunning
			instance.Status.ComponentsStatus = []opsterv1.ComponentStatus{}

			transport.RegisterResponder(
				http.MethodGet,
				fmt.Sprintf(
					"https://%s.%s.svc.cluster.local:9200/",
					instance.Spec.General.ServiceName,
					instance.Namespace,
				),
				httpmock.NewStringResponder(200, "OK").Times(2, failMessage),
			)

			transport.RegisterResponder(
				http.MethodHead,
				fmt.Sprintf(
					"https://%s.%s.svc.cluster.local:9200/",
					instance.Spec.General.ServiceName,
					instance.Namespace,
				),
				httpmock.NewStringResponder(200, "OK").Once(failMessage),
			)
		})

		When("snapshot repository exists in opensearch and is the same", func() {
			BeforeEach(func() {
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_snapshot/%s",
						instance.Spec.General.ServiceName,
						instance.Namespace,
						repoName,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.SnapshotRepositoryResponse{
						repoName: {
							Type: "fs",
							Settings: map[string]string{
								"foo": "bar",
							},
						},
					}).Once(failMessage),
				)
			})
			It("should do nothing", func() {
				_, err := reconciler.Reconcile()
				Expect(err).ToNot(HaveOccurred())
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
			})
		})
		When("snapshot repository exists in opensearch and is not the same", func() {
			BeforeEach(func() {
				recorder = record.NewFakeRecorder(1)
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_snapshot/%s",
						instance.Spec.General.ServiceName,
						instance.Namespace,
						repoName,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.SnapshotRepositoryResponse{
						repoName: {
							Type: "s3",
							Settings: map[string]string{
								"bar": "baz",
							},
						},
					}).Once(failMessage),
				)
				transport.RegisterResponder(
					http.MethodPut,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_snapshot/%s",
						instance.Spec.General.ServiceName,
						instance.Namespace,
						repoName,
					),
					httpmock.NewStringResponder(200, "OK").Once(failMessage),
				)
			})
			It("should update the repository", func() {
				go func() {
					defer GinkgoRecover()
					defer close(recorder.Events)
					_, err := reconciler.Reconcile()
					Expect(err).ToNot(HaveOccurred())
					// Confirm all responders have been called
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
				}()
				var events []string
				for msg := range recorder.Events {
					events = append(events, msg)
				}
				Expect(len(events)).To(Equal(1))
				Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s snapshot repository updated in opensearch", opensearchAPIUpdated)))
			})
		})
		When("snapshot repository doesn't exist in opensearch", func() {
			BeforeEach(func() {
				recorder = record.NewFakeRecorder(1)
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_snapshot/%s",
						instance.Spec.General.ServiceName,
						instance.Namespace,
						repoName,
					),
					httpmock.NewStringResponder(404, "does not exist").Once(failMessage),
				)
				transport.RegisterResponder(
					http.MethodPut,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_snapshot/%s",
						instance.Spec.General.ServiceName,
						instance.Namespace,
						repoName,
					),
					httpmock.NewStringResponder(200, "OK").Once(failMessage),
				)
			})
			It("should create the repository", func() {
				go func() {
					defer GinkgoRecover()
					defer close(recorder.Events)
					_, err := reconciler.Reconcile()
					Expect(err).ToNot(HaveOccurred())
					// Confirm all responders have been called
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
				}()
				var events []string
				for msg := range recorder.Events {
					events = append(events, msg)
				}
				Expect(len(events)).To(Equal(1))
				Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s snapshot repository created in opensearch", opensearchAPIUpdated)))
			})
		})
	})
})
