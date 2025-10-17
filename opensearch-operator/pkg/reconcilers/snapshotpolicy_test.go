package reconcilers

import (
	"context"
	"fmt"
	"k8s.io/utils/ptr"
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
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("snapshot policy reconciler", func() {
	var (
		transport  *httpmock.MockTransport
		reconciler *SnapshotPolicyReconciler
		instance   *opsterv1.OpensearchSnapshotPolicy
		recorder   *record.FakeRecorder
		mockClient *k8s.MockK8sClient

		// Objects
		cluster *opsterv1.OpenSearchCluster
	)

	BeforeEach(func() {
		mockClient = k8s.NewMockK8sClient(GinkgoT())
		transport = httpmock.NewMockTransport()
		transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
		instance = &opsterv1.OpensearchSnapshotPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-policy",
				Namespace: "test-policy",
				UID:       types.UID("testuid"),
			},
			Spec: opsterv1.OpensearchSnapshotPolicySpec{
				PolicyName: "test-policy",
				OpensearchRef: corev1.LocalObjectReference{
					Name: "test-cluster",
				},
			},
			Status: opsterv1.OpensearchSnapshotPolicyStatus{
				SnapshotPolicyName: "test-policy",
			},
		}

		cluster = &opsterv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "test-policy",
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
		reconciler = &SnapshotPolicyReconciler{
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
			mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(opsterv1.OpenSearchCluster{}, NotFoundError())
			recorder = record.NewFakeRecorder(1)
		})
		It("should wait he cluster to exist", func() {
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

	When("cluster is not ready", func() {
		BeforeEach(func() {
			recorder = record.NewFakeRecorder(1)
			mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(*cluster, nil)
		})
		It("should emit a unit test event and requeue", func() {
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
			recorder = record.NewFakeRecorder(1)

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

		When("cluster reference mismatch", func() {
			BeforeEach(func() {
				managedCluster := types.UID("different-uid")
				instance.Status.ManagedCluster = &managedCluster
			})

			It("should emit a unit test event and not requeue", func() {
				go func() {
					defer GinkgoRecover()
					defer close(recorder.Events)
					result, err := reconciler.Reconcile()
					Expect(err).To(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				}()
				var events []string
				for msg := range recorder.Events {
					events = append(events, msg)
				}
				Expect(len(events)).To(Equal(1))
				Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s cannot change the cluster a resource refers to", opensearchRefMismatch)))
			})
		})

		When("policy does not exist in opensearch", func() {
			BeforeEach(func() {
				mockClient.EXPECT().UdateObjectStatus(mock.Anything, mock.Anything).Return(nil)

				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_sm/policies/%s",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
						instance.Name,
					),
					httpmock.NewStringResponder(404, "Not Found").Once(),
				)
				transport.RegisterResponder(
					http.MethodPost,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_sm/policies/%s",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
						instance.Name,
					),
					httpmock.NewStringResponder(200, "OK").Once(),
				)
			})
			It("should create the policy, emit a unit test event, and requeue", func() {
				go func() {
					defer GinkgoRecover()
					defer close(recorder.Events)
					result, err := reconciler.Reconcile()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeTrue())
				}()
				var events []string
				for msg := range recorder.Events {
					events = append(events, msg)
				}
				Expect(len(events)).To(Equal(1))
				Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s policy successfully created in OpenSearch Cluster", opensearchAPIUpdated)))
			})
		})

		When("failed to get policy from opensearch api", func() {
			BeforeEach(func() {
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_sm/policies/%s",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
						instance.Name,
					),
					httpmock.NewErrorResponder(fmt.Errorf("failed to get policy")).Once(),
				)
			})
			It("should emit a unit test event, requeue, and return an error", func() {
				go func() {
					defer GinkgoRecover()
					defer close(recorder.Events)
					result, err := reconciler.Reconcile()
					Expect(err).To(HaveOccurred())
					Expect(result.Requeue).To(BeTrue())
				}()
				var events []string
				for msg := range recorder.Events {
					events = append(events, msg)
				}
				Expect(len(events)).To(Equal(1))
				Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s failed to get the snapshot policy from Opensearch API", opensearchAPIError)))
			})
		})

		Context("policy exists in opensearch", func() {
			BeforeEach(func() {
				instance.Spec.PolicyName = "test-policy-name"
				instance.Status.SnapshotPolicyName = "test-policy-name"

				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_sm/policies/%s",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
						instance.Spec.PolicyName,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetSnapshotPolicyResponse{
						PolicyId: "test-policy-name-sm-policy",
						Policy: requests.SnapshotPolicySpec{
							PolicyName:  "test-policy-name",
							Description: ptr.To("test-policy-name sample description"),
						},
					}).Once(),
				)
			})

			When("existing status is nil", func() {
				BeforeEach(func() {
					mockClient.EXPECT().UdateObjectStatus(mock.Anything, mock.Anything).Return(nil)
					instance.Status.ExistingSnapshotPolicy = nil
				})

				It("should emit a unit test event and requeue", func() {
					go func() {
						defer GinkgoRecover()
						defer close(recorder.Events)
						result, err := reconciler.Reconcile()
						Expect(err).ToNot(HaveOccurred())
						Expect(result.Requeue).To(BeTrue())
						// Confirm all responders have been called
						Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
					}()
					var events []string
					for msg := range recorder.Events {
						events = append(events, msg)
					}
					Expect(len(events)).To(Equal(1))
					Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s the Snapshot policy already exists in the OpenSearch cluster", opensearchSnapshotPolicyExists)))
				})
			})

			When("existing status is true", func() {
				BeforeEach(func() {
					mockClient.EXPECT().UdateObjectStatus(mock.Anything, mock.Anything).Return(nil)
					instance.Status.ExistingSnapshotPolicy = ptr.To(true)
				})

				It("should emit a unit test event and requeue", func() {
					go func() {
						defer GinkgoRecover()
						defer close(recorder.Events)
						result, err := reconciler.Reconcile()
						Expect(err).ToNot(HaveOccurred())
						Expect(result.Requeue).To(BeTrue())
						// Confirm all responders have been called
						Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
					}()
					var events []string
					for msg := range recorder.Events {
						events = append(events, msg)
					}
					Expect(len(events)).To(Equal(1))
					Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s the Snapshot policy already exists in the OpenSearch cluster", opensearchSnapshotPolicyExists)))
				})
			})

			Context("existing status is false", func() {
				BeforeEach(func() {
					instance.Status.ExistingSnapshotPolicy = ptr.To(false)
				})

				When("policy is the same", func() {
					BeforeEach(func() {
						instance.Spec.PolicyName = "test-policy-name"
						instance.Spec.Description = ptr.To("test-policy-name sample description")
					})
					It("should emit a unit test event and requeue", func() {
						go func() {
							defer GinkgoRecover()
							defer close(recorder.Events)
							result, err := reconciler.Reconcile()
							Expect(err).ToNot(HaveOccurred())
							Expect(result.Requeue).To(BeTrue())
							// Confirm all responders have been called
							Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
						}()
						var events []string
						for msg := range recorder.Events {
							events = append(events, msg)
						}
						Expect(len(events)).To(Equal(1))
						Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s policy is in sync", opensearchAPIUnchanged)))
					})
				})

				When("policy is not the same", func() {
					BeforeEach(func() {
						instance.Spec.PolicyName = "test-policy-name-2"
						instance.Spec.Description = ptr.To("test-policy-name sample description 2")

						transport.RegisterResponder(
							http.MethodGet,
							fmt.Sprintf(
								"https://%s.%s.svc.cluster.local:9200/_plugins/_sm/policies/%s",
								cluster.Spec.General.ServiceName,
								cluster.Namespace,
								instance.Spec.PolicyName,
							),
							httpmock.NewJsonResponderOrPanic(200, map[string]interface{}{
								"policy": map[string]interface{}{
									"name":        instance.Spec.PolicyName,
									"description": "changed description",
								},
							}).Once(),
						)

						transport.RegisterResponder(
							http.MethodPut,
							fmt.Sprintf(
								"https://%s.%s.svc.cluster.local:9200/_plugins/_sm/policies/?if_seq_no=0&if_primary_term=0",
								cluster.Spec.General.ServiceName,
								cluster.Namespace,
							),
							httpmock.NewStringResponder(200, "OK").Once(),
						)

					})
					It("should update snapshot policy, emit a unit test event, and requeue", func() {
						go func() {
							defer GinkgoRecover()
							defer close(recorder.Events)
							result, err := reconciler.Reconcile()
							Expect(err).ToNot(HaveOccurred())
							Expect(result.Requeue).To(BeTrue())
							// Confirm all responders have been called
							Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders()))
						}()
						var events []string
						for msg := range recorder.Events {
							events = append(events, msg)
						}
						Expect(len(events)).To(Equal(1))
						Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s policy updated in opensearch", opensearchAPIUpdated)))
					})
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
				instance.Status.ExistingSnapshotPolicy = ptr.To(true)
			})
			It("should do nothing and exit", func() {
				Expect(reconciler.Delete()).To(Succeed())
			})
		})

		Context("existing status is false", func() {
			BeforeEach(func() {
				instance.Status.ExistingSnapshotPolicy = ptr.To(false)
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

			Context("cluster is ready", func() {
				// extraContextCalls := 1
				BeforeEach(func() {
					cluster.Status.Phase = opsterv1.PhaseRunning
					cluster.Status.ComponentsStatus = []opsterv1.ComponentStatus{}
					mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(*cluster, nil)
					recorder = record.NewFakeRecorder(1)

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

				When("policy does not exist", func() {
					BeforeEach(func() {
						transport.RegisterResponder(
							http.MethodDelete,
							fmt.Sprintf(
								"https://%s.%s.svc.cluster.local:9200/_plugins/_sm/policies/%s",
								cluster.Spec.General.ServiceName,
								cluster.Namespace,
								instance.Name,
							),
							httpmock.NewStringResponder(404, "does not exist").Once(failMessage),
						)
					})
					It("should do nothing and exit", func() {
						Expect(reconciler.Delete()).NotTo(Succeed())
					})
				})

				When("policy does exist", func() {
					BeforeEach(func() {
						transport.RegisterResponder(
							http.MethodDelete,
							fmt.Sprintf(
								"https://%s.%s.svc.cluster.local:9200/_plugins/_sm/policies/%s",
								cluster.Spec.General.ServiceName,
								cluster.Namespace,
								instance.Name,
							),
							httpmock.NewStringResponder(200, "OK").Once(failMessage),
						)
					})
					It("should delete the policy", func() {
						Expect(reconciler.Delete()).To(Succeed())
					})
				})
			})
		})
	})

	Context("CreateSnapshotPolicy", func() {
		When("deletion schedule is different from creation schedule", func() {
			BeforeEach(func() {
				instance.Spec.Creation = opsterv1.SnapshotCreation{
					Schedule: opsterv1.CronSchedule{
						Cron: opsterv1.CronExpression{
							Expression: "0 6 * * *", // 6:00 AM
							Timezone:   "UTC",
						},
					},
					TimeLimit: ptr.To("2h"),
				}
				instance.Spec.Deletion = &opsterv1.SnapshotDeletion{
					Schedule: &opsterv1.CronSchedule{
						Cron: opsterv1.CronExpression{
							Expression: "0 8 * * *", // 8:00 AM
							Timezone:   "UTC",
						},
					},
					TimeLimit: ptr.To("2h"),
					DeleteCondition: &opsterv1.SnapshotDeleteCondition{
						MaxAge:   ptr.To("180d"),
						MinCount: ptr.To(1),
					},
				}
			})

			It("should use deletion schedule, not creation schedule", func() {
				policy, err := reconciler.CreateSnapshotPolicy()
				Expect(err).NotTo(HaveOccurred())
				Expect(policy).NotTo(BeNil())
				Expect(policy.Deletion).NotTo(BeNil())
				Expect(policy.Deletion.Schedule).NotTo(BeNil())
				Expect(policy.Deletion.Schedule.Cron.Expression).To(Equal("0 8 * * *"))
				Expect(policy.Deletion.Schedule.Cron.Timezone).To(Equal("UTC"))
			})
		})

		When("deletion schedule is nil", func() {
			BeforeEach(func() {
				instance.Spec.Creation = opsterv1.SnapshotCreation{
					Schedule: opsterv1.CronSchedule{
						Cron: opsterv1.CronExpression{
							Expression: "0 6 * * *",
							Timezone:   "UTC",
						},
					},
					TimeLimit: ptr.To("2h"),
				}
				instance.Spec.Deletion = &opsterv1.SnapshotDeletion{
					Schedule:  nil, // No schedule
					TimeLimit: ptr.To("2h"),
					DeleteCondition: &opsterv1.SnapshotDeleteCondition{
						MaxAge:   ptr.To("180d"),
						MinCount: ptr.To(1),
					},
				}
			})

			It("should not set deletion schedule", func() {
				policy, err := reconciler.CreateSnapshotPolicy()
				Expect(err).NotTo(HaveOccurred())
				Expect(policy).NotTo(BeNil())
				Expect(policy.Deletion).NotTo(BeNil())
				Expect(policy.Deletion.Schedule).To(BeNil())
			})
		})
	})
})
