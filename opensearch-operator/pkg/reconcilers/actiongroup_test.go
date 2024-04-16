package reconcilers

import (
	"context"
	"fmt"
	"net/http"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/mocks/github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("actiongroup reconciler", func() {
	var (
		transport  *httpmock.MockTransport
		reconciler *ActionGroupReconciler
		instance   *opsterv1.OpensearchActionGroup
		recorder   *record.FakeRecorder

		// Objects
		cluster    *opsterv1.OpenSearchCluster
		mockClient *k8s.MockK8sClient
	)

	BeforeEach(func() {
		mockClient = k8s.NewMockK8sClient(GinkgoT())
		transport = httpmock.NewMockTransport()
		transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
		instance = &opsterv1.OpensearchActionGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-actiongroup",
				Namespace: "test-actiongroup",
				UID:       "testuid",
			},
			Spec: opsterv1.OpensearchActionGroupSpec{
				OpensearchRef: corev1.LocalObjectReference{
					Name: "test-cluster",
				},
				AllowedActions: []string{
					"test_cluster_permission",
				},
				Type:        "test",
				Description: "test-description",
			},
		}

		cluster = &opsterv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "test-actiongroup",
			},
			Spec: opsterv1.ClusterSpec{
				General: opsterv1.GeneralConfig{
					ServiceName: "test-cluster",
					HttpPort:    9200,
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
		}
	})

	JustBeforeEach(func() {
		options := ReconcilerOptions{}
		options.apply(WithOSClientTransport(transport), WithUpdateStatus(false))
		reconciler = &ActionGroupReconciler{
			client:            mockClient,
			ctx:               context.Background(),
			ReconcilerOptions: options,
			recorder:          recorder,
			instance:          instance,
			logger:            log.FromContext(context.Background()),
		}
	})

	When("cluster doesn't exist", func() {
		BeforeEach(func() {
			instance.Spec.OpensearchRef.Name = "doesnotexist"
			recorder = record.NewFakeRecorder(1)
		})
		It("should wait for the cluster to exist", func() {
			mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(opsterv1.OpenSearchCluster{}, NotFoundError())
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
			Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s waiting for opensearch cluster to exist", opensearchPending)))
		})
	})

	When("cluster doesn't match status", func() {
		BeforeEach(func() {
			uid := types.UID("someuid")
			instance.Status.ManagedCluster = &uid
			mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(*cluster, nil)
			recorder = record.NewFakeRecorder(1)
		})
		It("should error", func() {
			go func() {
				defer GinkgoRecover()
				defer close(recorder.Events)
				_, err := reconciler.Reconcile()
				Expect(err).To(HaveOccurred())
			}()
			var events []string
			for msg := range recorder.Events {
				events = append(events, msg)
			}
			Expect(len(events)).To(Equal(1))
			Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s cannot change the cluster an actiongroup refers to", opensearchRefMismatch)))
		})
	})

	When("cluster is not ready", func() {
		BeforeEach(func() {
			recorder = record.NewFakeRecorder(1)
			mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(*cluster, nil)
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
			cluster.Status.Phase = opsterv1.PhaseRunning
			cluster.Status.ComponentsStatus = []opsterv1.ComponentStatus{}
			mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(*cluster, nil)

			transport.RegisterResponder(
				http.MethodGet,
				fmt.Sprintf(
					"https://%s.%s.svc.cluster.local:9200/",
					cluster.Spec.General.ServiceName,
					cluster.Namespace,
				),
				httpmock.NewStringResponder(200, "OK").Times(2, failMessage),
			)

			transport.RegisterResponder(
				http.MethodHead,
				fmt.Sprintf(
					"https://%s.%s.svc.cluster.local:9200/",
					cluster.Spec.General.ServiceName,
					cluster.Namespace,
				),
				httpmock.NewStringResponder(200, "OK").Once(failMessage),
			)
		})

		When("existing status is true", func() {
			BeforeEach(func() {
				instance.Status.ExistingActionGroup = pointer.Bool(true)
			})

			It("should do nothing", func() {
				_, err := reconciler.Reconcile()
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("existing status is nil", func() {
			localExtraCalls := 4
			BeforeEach(func() {
				actionGroupRequest := requests.ActionGroup{
					AllowedActions: []string{
						"test_cluster_permission",
					},
					Type:        "test",
					Description: "test-description",
				}
				recorder = record.NewFakeRecorder(1)
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					httpmock.NewStringResponder(200, "OK").Times(4, failMessage),
				)
				transport.RegisterResponder(
					http.MethodHead,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					httpmock.NewStringResponder(200, "OK").Times(2, failMessage),
				)
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/actiongroups/%s",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
						instance.Name,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetActionGroupResponse{
						instance.Name: actionGroupRequest,
					}).Then(
						httpmock.NewStringResponder(404, "does not exist"),
					).Then(
						httpmock.NewNotFoundResponder(failMessage),
					),
				)
			})

			It("should do nothing and emit a unit test event", func() {
				go func() {
					defer GinkgoRecover()
					defer close(recorder.Events)
					_, err := reconciler.Reconcile()
					Expect(err).ToNot(HaveOccurred())
					_, err = reconciler.Reconcile()
					Expect(err).ToNot(HaveOccurred())
					// Confirm all responders have been called
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + localExtraCalls))
				}()
				var events []string
				for msg := range recorder.Events {
					events = append(events, msg)
				}
				Expect(len(events)).To(Equal(2))
				Expect(events[0]).To(Equal("Normal UnitTest exists is true"))
				Expect(events[1]).To(Equal("Normal UnitTest exists is false"))
			})
		})

		When("existing status is true", func() {
			BeforeEach(func() {
				instance.Status.ExistingActionGroup = pointer.Bool(true)
			})
			It("should do nothing", func() {
				_, err := reconciler.Reconcile()
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("existing status is false", func() {
			BeforeEach(func() {
				instance.Status.ExistingActionGroup = pointer.Bool(false)
			})

			When("actiongroup exists in opensearch and is the same", func() {
				BeforeEach(func() {
					actionGroupRequest := requests.ActionGroup{
						AllowedActions: []string{
							"test_cluster_permission",
						},
						Type:        "test",
						Description: "test-description",
					}
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/actiongroups/%s",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
							instance.Name,
						),
						httpmock.NewJsonResponderOrPanic(200, responses.GetActionGroupResponse{
							instance.Name: actionGroupRequest,
						}).Once(failMessage),
					)
				})
				It("should do nothing", func() {
					_, err := reconciler.Reconcile()
					Expect(err).ToNot(HaveOccurred())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
				})
			})
			When("actiongroup exists in opensearch and is not the same", func() {
				BeforeEach(func() {
					recorder = record.NewFakeRecorder(1)
					actionGroupRequest := requests.ActionGroup{
						AllowedActions: []string{
							"test_cluster_permission",
						},
						Type:        "some other test",
						Description: "test-description",
					}
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/actiongroups/%s",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
							instance.Name,
						),
						httpmock.NewJsonResponderOrPanic(200, responses.GetActionGroupResponse{
							instance.Name: actionGroupRequest,
						}).Once(failMessage),
					)
					transport.RegisterResponder(
						http.MethodPut,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/actiongroups/%s",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
							instance.Name,
						),
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
				})
				It("should update the actiongroup", func() {
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
					Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s actiongroup updated in opensearch", opensearchAPIUpdated)))
				})
			})
			When("actiongroup doesn't exist in opensearch", func() {
				BeforeEach(func() {
					recorder = record.NewFakeRecorder(1)
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/actiongroups/%s",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
							instance.Name,
						),
						httpmock.NewStringResponder(404, "does not exist").Once(failMessage),
					)
					transport.RegisterResponder(
						http.MethodPut,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/actiongroups/%s",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
							instance.Name,
						),
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
				})
				It("should create the actiongroup", func() {
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
					Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s actiongroup updated in opensearch", opensearchAPIUpdated)))
				})
			})
		})
	})

	Context("deletions", func() {
		When("existing status is nil", func() {
			It("should do nothing and exit", func() {
				Expect(reconciler.Delete()).To(Succeed())
			})
		})

		When("existing status is true", func() {
			BeforeEach(func() {
				instance.Status.ExistingActionGroup = pointer.Bool(true)
			})
			It("should do nothing and exit", func() {
				Expect(reconciler.Delete()).To(Succeed())
			})
		})

		Context("existing status is false", func() {
			BeforeEach(func() {
				instance.Status.ExistingActionGroup = pointer.Bool(false)
			})

			When("cluster does not exist", func() {
				BeforeEach(func() {
					instance.Spec.OpensearchRef.Name = "doesnotexist"
					mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(opsterv1.OpenSearchCluster{}, NotFoundError())
				})
				It("should do nothing and exit", func() {
					Expect(reconciler.Delete()).To(Succeed())
				})
			})

			When("actiongroup does not exist", func() {
				BeforeEach(func() {
					mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(*cluster, nil)
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						httpmock.NewStringResponder(200, "OK").Times(2, failMessage),
					)
					transport.RegisterResponder(
						http.MethodHead,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/actiongroups/%s",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
							instance.Name,
						),
						httpmock.NewStringResponder(404, "does not exist").Once(failMessage),
					)
				})
				It("should do nothing and exit", func() {
					Expect(reconciler.Delete()).To(Succeed())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + 1))
				})
			})
			When("actiongroup does exist", func() {
				BeforeEach(func() {
					mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(*cluster, nil)
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						httpmock.NewStringResponder(200, "OK").Times(2, failMessage),
					)
					transport.RegisterResponder(
						http.MethodHead,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/actiongroups/%s",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
							instance.Name,
						),
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
					transport.RegisterResponder(
						http.MethodDelete,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/actiongroups/%s",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
							instance.Name,
						),
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
				})
				It("should delete the actiongroup", func() {
					Expect(reconciler.Delete()).To(Succeed())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + 1))
				})
			})
		})
	})
})
