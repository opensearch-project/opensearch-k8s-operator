package reconcilers

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/utils/ptr"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/mocks/github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
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

var _ = Describe("ism policy reconciler", func() {
	var (
		transport  *httpmock.MockTransport
		reconciler *IsmPolicyReconciler
		instance   *opsterv1.OpenSearchISMPolicy
		recorder   *record.FakeRecorder
		mockClient *k8s.MockK8sClient

		// Objects
		cluster    *opsterv1.OpenSearchCluster
		clusterUrl string
	)

	BeforeEach(func() {
		mockClient = k8s.NewMockK8sClient(GinkgoT())
		transport = httpmock.NewMockTransport()
		transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
		instance = &opsterv1.OpenSearchISMPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-policy",
				Namespace: "test-policy",
				UID:       types.UID("testuid"),
			},
			Spec: opsterv1.OpenSearchISMPolicySpec{
				PolicyID: "test-policy",
				OpensearchRef: corev1.LocalObjectReference{
					Name: "test-cluster",
				},
			},
			Status: opsterv1.OpensearchISMPolicyStatus{
				PolicyId: "test-policy",
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
		clusterUrl = fmt.Sprintf("%s/", helpers.ClusterURL(cluster))
		// Mock admin credentials secret for all tests (available when CreateClientForCluster is invoked)
		adminSecret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster-admin-password",
				Namespace: "test-policy",
			},
			Data: map[string][]byte{
				"username": []byte("admin"),
				"password": []byte("admin"),
			},
		}
		mockClient.On("GetSecret", "test-cluster-admin-password", "test-policy").Return(func(string, string) corev1.Secret {
			return adminSecret
		}, nil).Maybe()
	})

	JustBeforeEach(func() {
		options := ReconcilerOptions{}
		options.apply(WithOSClientTransport(transport), WithUpdateStatus(false))
		reconciler = &IsmPolicyReconciler{
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
		It("should wait for the cluster to exist", func() {
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
				clusterUrl,
				httpmock.NewStringResponder(200, "OK").Times(2, failMessage),
			)

			transport.RegisterResponder(
				http.MethodHead,
				clusterUrl,
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

		Context("policy does not exist in opensearch", func() {
			BeforeEach(func() {
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"%s_plugins/_ism/policies/%s",
						clusterUrl,
						instance.Name,
					),
					httpmock.NewStringResponder(404, "Not Found").Once(),
				)

				transport.RegisterResponder(
					http.MethodPut,
					fmt.Sprintf(
						"%s_plugins/_ism/policies/%s",
						clusterUrl,
						instance.Name,
					),
					httpmock.NewStringResponder(200, "OK").Once(),
				)
			})

			When("apply ism policy to existing indices is false", func() {
				BeforeEach(func() {
					instance.Spec.ApplyToExistingIndices = ptr.To(false)
					mockClient.EXPECT().UdateObjectStatus(mock.Anything, mock.Anything).Return(nil)
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

			Context("applyToExistingIndices is true", func() {
				indexName := "test-index-1"
				BeforeEach(func() {
					instance.Spec.ApplyToExistingIndices = ptr.To(true)
					instance.Spec.ISMTemplate = &opsterv1.ISMTemplate{
						IndexPatterns: []string{"test-*"},
					}
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"%s_cat/indices/test-*",
							clusterUrl,
						),
						httpmock.NewJsonResponderOrPanic(200, []map[string]interface{}{
							{"index": indexName},
						}),
					)
				})

				When("successfully applied applied policy to indices", func() {
					BeforeEach(func() {
						mockClient.EXPECT().UdateObjectStatus(mock.Anything, mock.Anything).Return(nil)
						transport.RegisterResponder(
							http.MethodPost,
							fmt.Sprintf(
								"%s_plugins/_ism/add/%s",
								clusterUrl,
								indexName,
							),
							httpmock.NewStringResponder(200, "OK").Once(),
						)
					})

					It("should create the policy, apply it to existing indices, emit a unit test event, and requeue", func() {
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
						Expect(len(events)).To(Equal(2))
						Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s ISM policy applied to existing indices", opensearchAPIUpdated)))
						Expect(events[1]).To(Equal(fmt.Sprintf("Normal %s policy successfully created in OpenSearch Cluster", opensearchAPIUpdated)))
					})
				})

				When("failed to get indices from opensearch api", func() {
					BeforeEach(func() {
						transport.RegisterResponder(
							http.MethodGet,
							fmt.Sprintf(
								"%s_cat/indices/test-*",
								clusterUrl,
							),
							httpmock.NewErrorResponder(fmt.Errorf("failed to get indices")).Once(),
						)
					})
					It("should emit a unit test event and requeue", func() {
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
						Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s failed to apply policy to existing indices", opensearchAPIError)))
					})
				})

				When("failed to apply policy to existing indices", func() {
					BeforeEach(func() {
						transport.RegisterResponder(
							http.MethodPost,
							fmt.Sprintf(
								"%s_plugins/_ism/add/test-index-1",
								clusterUrl,
							),
							httpmock.NewErrorResponder(fmt.Errorf("failed to apply policy")).Once(),
						)
					})

					It("should emit a unit test event and requeue", func() {
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
						Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s failed to apply policy to existing indices", opensearchAPIError)))
					})
				})
			})
		})

		When("failed to get policy from opensearch api", func() {
			BeforeEach(func() {
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"%s_plugins/_ism/policies/%s",
						clusterUrl,
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
				Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s failed to get the ism policy from Opensearch API", opensearchAPIError)))
			})
		})

		Context("policy exists in opensearch", func() {
			BeforeEach(func() {
				instance.Spec.PolicyID = "test-policy-id"
				instance.Status.PolicyId = "test-policy-id"

				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"%s_plugins/_ism/policies/%s",
						clusterUrl,
						instance.Spec.PolicyID,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetISMPolicyResponse{
						PolicyID: "test-policy-id",
						Policy: requests.ISMPolicySpec{
							DefaultState: "test-state",
							Description:  "test-policy",
						},
					}).Once(),
				)
			})

			When("existing status is nil", func() {
				BeforeEach(func() {
					mockClient.EXPECT().UdateObjectStatus(mock.Anything, mock.Anything).Return(nil)
					instance.Status.ExistingISMPolicy = nil
				})

				It("should emit a unit test event and requeue", func() {
					go func() {
						defer GinkgoRecover()
						defer close(recorder.Events)
						result, err := reconciler.Reconcile()
						Expect(err).ToNot(HaveOccurred())
						Expect(result.Requeue).To(BeTrue())
						Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
					}()
					var events []string
					for msg := range recorder.Events {
						events = append(events, msg)
					}
					Expect(len(events)).To(Equal(1))
					Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s the ISM policy already exists in the OpenSearch cluster", opensearchIsmPolicyExists)))
				})
			})

			When("existing status is true", func() {
				BeforeEach(func() {
					mockClient.EXPECT().UdateObjectStatus(mock.Anything, mock.Anything).Return(nil)
					instance.Status.ExistingISMPolicy = ptr.To(true)
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
					Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s the ISM policy already exists in the OpenSearch cluster", opensearchIsmPolicyExists)))
				})
			})

			Context("existing status is false", func() {
				BeforeEach(func() {
					instance.Status.ExistingISMPolicy = ptr.To(false)
				})

				When("policy is the same", func() {
					BeforeEach(func() {
						instance.Spec.DefaultState = "test-state"
						instance.Spec.Description = "test-policy"
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
						instance.Spec.DefaultState = "test-state2"
						instance.Spec.Description = "test-policy2"

						transport.RegisterResponder(
							http.MethodPut,
							fmt.Sprintf(
								"%s_plugins/_ism/policies/%s",
								clusterUrl,
								instance.Spec.PolicyID,
							),
							httpmock.NewStringResponder(200, "OK").Once(),
						)
					})

					It("should update ism policy, emit a unit test event, and requeue", func() {
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
						Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s policy updated in opensearch", opensearchAPIUpdated)))
					})
				})
			})
		})
	})

	Context("CreateISMPolicy Shrink Action Validation", func() {
		var (
			originalInstanceSpec opsterv1.OpenSearchISMPolicySpec
		)

		BeforeEach(func() {
			recorder = record.NewFakeRecorder(1)

			options := ReconcilerOptions{}
			options.apply(WithOSClientTransport(transport), WithUpdateStatus(false))
			reconciler = &IsmPolicyReconciler{
				client:            mockClient,
				ctx:               context.Background(),
				ReconcilerOptions: options,
				recorder:          recorder,
				instance:          instance,
				logger:            log.FromContext(context.Background()),
			}

			originalInstanceSpec = instance.Spec
		})

		AfterEach(func() {
			instance.Spec = originalInstanceSpec
		})

		When("a Shrink action is configured correctly with NumNewShards", func() {
			BeforeEach(func() {
				instance.Spec.States = []opsterv1.State{
					{
						Name: "hot",
						Actions: []opsterv1.Action{
							{
								Shrink: &opsterv1.Shrink{
									NumNewShards: ptr.To(1),
								},
							},
						},
					},
				}
			})
			It("should create the policy without error and set NumNewShards", func() {
				policy, err := reconciler.CreateISMPolicy()
				Expect(err).NotTo(HaveOccurred())
				Expect(policy).ToNot(BeNil())
				Expect(policy.States).To(HaveLen(1))
				Expect(policy.States[0].Actions).To(HaveLen(1))
				Expect(policy.States[0].Actions[0].Shrink).ToNot(BeNil())
				Expect(policy.States[0].Actions[0].Shrink.NumNewShards).To(Equal(ptr.To(1)))
				Expect(policy.States[0].Actions[0].Shrink.MaxShardSize).To(BeNil())
				Expect(policy.States[0].Actions[0].Shrink.PercentageOfSourceShards).To(BeNil())
			})
		})

		When("a Shrink action is configured correctly with MaxShardSize", func() {
			BeforeEach(func() {
				instance.Spec.States = []opsterv1.State{
					{
						Name: "hot",
						Actions: []opsterv1.Action{
							{
								Shrink: &opsterv1.Shrink{
									MaxShardSize: ptr.To("1gb"),
								},
							},
						},
					},
				}
			})
			It("should create the policy without error and set MaxShardSize", func() {
				policy, err := reconciler.CreateISMPolicy()
				Expect(err).NotTo(HaveOccurred())
				Expect(policy).ToNot(BeNil())
				Expect(policy.States).To(HaveLen(1))
				Expect(policy.States[0].Actions).To(HaveLen(1))
				Expect(policy.States[0].Actions[0].Shrink).ToNot(BeNil())
				Expect(policy.States[0].Actions[0].Shrink.NumNewShards).To(BeNil())
				Expect(policy.States[0].Actions[0].Shrink.MaxShardSize).To(Equal(ptr.To("1gb")))
				Expect(policy.States[0].Actions[0].Shrink.PercentageOfSourceShards).To(BeNil())
			})
		})

		When("a Shrink action is configured correctly with PercentageOfSourceShards", func() {
			BeforeEach(func() {
				instance.Spec.States = []opsterv1.State{
					{
						Name: "hot",
						Actions: []opsterv1.Action{
							{
								Shrink: &opsterv1.Shrink{
									PercentageOfSourceShards: ptr.To[int64](50),
								},
							},
						},
					},
				}
			})
			It("should create the policy without error and set PercentageOfSourceShards", func() {
				policy, err := reconciler.CreateISMPolicy()
				Expect(err).NotTo(HaveOccurred())
				Expect(policy).ToNot(BeNil())
				Expect(policy.States).To(HaveLen(1))
				Expect(policy.States[0].Actions).To(HaveLen(1))
				Expect(policy.States[0].Actions[0].Shrink).ToNot(BeNil())
				Expect(policy.States[0].Actions[0].Shrink.NumNewShards).To(BeNil())
				Expect(policy.States[0].Actions[0].Shrink.MaxShardSize).To(BeNil())
				Expect(policy.States[0].Actions[0].Shrink.PercentageOfSourceShards).To(Equal(ptr.To[int64](50)))
			})
		})

		When("a Shrink action is configured with NumNewShards and MaxShardSize", func() {
			BeforeEach(func() {
				instance.Spec.States = []opsterv1.State{
					{
						Name: "hot",
						Actions: []opsterv1.Action{
							{
								Shrink: &opsterv1.Shrink{
									NumNewShards: ptr.To(1),
									MaxShardSize: ptr.To("1gb"),
								},
							},
						},
					},
				}
			})
			It("should return an error", func() {
				_, err := reconciler.CreateISMPolicy()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("maxShardSize can't exist with NumNewShards or PercentageOfSourceShards"))
			})
		})

		When("a Shrink action is configured with NumNewShards and PercentageOfSourceShards", func() {
			BeforeEach(func() {
				instance.Spec.States = []opsterv1.State{
					{
						Name: "hot",
						Actions: []opsterv1.Action{
							{
								Shrink: &opsterv1.Shrink{
									NumNewShards:             ptr.To(1),
									PercentageOfSourceShards: ptr.To[int64](50),
								},
							},
						},
					},
				}
			})
			It("should return an error", func() {
				_, err := reconciler.CreateISMPolicy()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("numNewShards can't exist with MaxShardSize or PercentageOfSourceShards"))
			})
		})

		When("a Shrink action is configured with MaxShardSize and PercentageOfSourceShards", func() {
			BeforeEach(func() {
				instance.Spec.States = []opsterv1.State{
					{
						Name: "hot",
						Actions: []opsterv1.Action{
							{
								Shrink: &opsterv1.Shrink{
									MaxShardSize:             ptr.To("1gb"),
									PercentageOfSourceShards: ptr.To[int64](50),
								},
							},
						},
					},
				}
			})
			It("should return an error", func() {
				_, err := reconciler.CreateISMPolicy()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("maxShardSize can't exist with NumNewShards or PercentageOfSourceShards"))
			})
		})

		When("a Shrink action is configured with none of the required parameters", func() {
			BeforeEach(func() {
				instance.Spec.States = []opsterv1.State{
					{
						Name: "hot",
						Actions: []opsterv1.Action{
							{
								Shrink: &opsterv1.Shrink{
									// No fields set
								},
							},
						},
					},
				}
			})
			It("should return an error", func() {
				_, err := reconciler.CreateISMPolicy()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("either of MaxShardSize or NumNewShards or PercentageOfSourceShards is required"))
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
				instance.Status.ExistingISMPolicy = ptr.To(true)
			})

			It("should do nothing and exit", func() {
				Expect(reconciler.Delete()).To(Succeed())
			})
		})

		Context("existing status is false", func() {
			BeforeEach(func() {
				instance.Status.ExistingISMPolicy = ptr.To(false)
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
						clusterUrl,
						httpmock.NewStringResponder(200, "OK").Times(2, failMessage),
					)

					transport.RegisterResponder(
						http.MethodHead,
						clusterUrl,
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
				})

				When("policy does not exist", func() {
					BeforeEach(func() {
						transport.RegisterResponder(
							http.MethodDelete,
							fmt.Sprintf(
								"%s_plugins/_ism/policies/%s",
								clusterUrl,
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
								"%s_plugins/_ism/policies/%s",
								clusterUrl,
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
})
