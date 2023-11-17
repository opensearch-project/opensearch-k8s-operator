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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("indextemplate reconciler", func() {
	var (
		transport  *httpmock.MockTransport
		reconciler *IndexTemplateReconciler
		instance   *opsterv1.OpensearchIndexTemplate
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
		instance = &opsterv1.OpensearchIndexTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-indextemplate",
				Namespace: "test-indextemplate",
				UID:       "testuid",
			},
			Spec: opsterv1.OpensearchIndexTemplateSpec{
				OpensearchRef: corev1.LocalObjectReference{
					Name: "test-cluster",
				},
				Name:          "my-template",
				IndexPatterns: []string{"my-logs-*"},
				Template: opsterv1.OpensearchIndexSpec{
					Settings: &apiextensionsv1.JSON{},
					Mappings: &apiextensionsv1.JSON{},
					Aliases:  make(map[string]opsterv1.OpensearchIndexAliasSpec),
				},
				ComposedOf: []string{},
				Priority:   0,
				Version:    0,
				Meta:       &apiextensionsv1.JSON{},
			},
		}

		cluster = &opsterv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "test-indextemplate",
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
		clusterUrl = fmt.Sprintf("https://%s.%s.svc.cluster.local:9200/", cluster.Spec.General.ServiceName, cluster.Namespace)
	})

	JustBeforeEach(func() {
		options := ReconcilerOptions{}
		options.apply(WithOSClientTransport(transport), WithUpdateStatus(false))
		reconciler = &IndexTemplateReconciler{
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
			Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s cannot change the cluster an index template refers to", opensearchRefMismatch)))
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
				clusterUrl,
				httpmock.NewStringResponder(200, "OK").Times(2, failMessage),
			)
			transport.RegisterResponder(
				http.MethodHead,
				clusterUrl,
				httpmock.NewStringResponder(200, "OK").Once(failMessage),
			)
		})

		When("existing status is true", func() {
			BeforeEach(func() {
				instance.Status.ExistingIndexTemplate = pointer.Bool(true)
			})

			It("should do nothing", func() {
				_, err := reconciler.Reconcile()
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("existing status is nil", func() {
			localExtraCalls := 4
			BeforeEach(func() {
				recorder = record.NewFakeRecorder(1)
				transport.RegisterResponder(
					http.MethodGet,
					clusterUrl,
					httpmock.NewStringResponder(200, "OK").Times(4, failMessage),
				)
				transport.RegisterResponder(
					http.MethodHead,
					clusterUrl,
					httpmock.NewStringResponder(200, "OK").Times(2, failMessage),
				)
				transport.RegisterResponder(
					http.MethodHead,
					fmt.Sprintf("%s_index_template/my-template", clusterUrl),
					httpmock.NewJsonResponderOrPanic(200, "OK").Then(
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
				instance.Status.ExistingIndexTemplate = pointer.Bool(true)
			})

			It("should do nothing", func() {
				_, err := reconciler.Reconcile()
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("existing status is false", func() {
			BeforeEach(func() {
				instance.Status.ExistingIndexTemplate = pointer.Bool(false)
			})

			When("indextemplate exists in opensearch and is the same", func() {
				BeforeEach(func() {
					response := responses.GetIndexTemplatesResponse{
						IndexTemplates: make([]responses.IndexTemplate, 1),
					}
					response.IndexTemplates[0] = responses.IndexTemplate{
						Name: "my-template",
						IndexTemplate: requests.IndexTemplate{
							IndexPatterns: []string{"my-logs-*"},
							Template: requests.Index{
								Settings: &apiextensionsv1.JSON{},
								Mappings: &apiextensionsv1.JSON{},
								Aliases:  make(map[string]requests.IndexAlias),
							},
							ComposedOf: []string{},
							Priority:   0,
							Version:    0,
							Meta:       &apiextensionsv1.JSON{},
						},
					}

					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf("%s_index_template/my-template", clusterUrl),
						httpmock.NewJsonResponderOrPanic(200, response).Once(failMessage),
					)
				})

				It("should do nothing", func() {
					_, err := reconciler.Reconcile()
					Expect(err).ToNot(HaveOccurred())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
				})
			})

			When("indextemplate exists in opensearch and is not the same", func() {
				BeforeEach(func() {
					recorder = record.NewFakeRecorder(1)

					response := responses.GetIndexTemplatesResponse{
						IndexTemplates: make([]responses.IndexTemplate, 1),
					}
					response.IndexTemplates[0] = responses.IndexTemplate{
						Name: "my-template",
						IndexTemplate: requests.IndexTemplate{
							IndexPatterns: []string{"my-logs-*"},
							Template: requests.Index{
								Settings: &apiextensionsv1.JSON{},
								Mappings: &apiextensionsv1.JSON{},
								Aliases:  make(map[string]requests.IndexAlias),
							},
							ComposedOf: []string{},
							Priority:   100,
							Version:    100,
							Meta:       &apiextensionsv1.JSON{},
						},
					}

					indexTemplateUrl := fmt.Sprintf("%s_index_template/my-template", clusterUrl)
					transport.RegisterResponder(
						http.MethodGet,
						indexTemplateUrl,
						httpmock.NewJsonResponderOrPanic(200, response).Once(failMessage),
					)
					transport.RegisterResponder(
						http.MethodPut,
						indexTemplateUrl,
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
				})

				It("should update the indextemplate", func() {
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
					Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s index template updated in opensearch", opensearchAPIUpdated)))
				})
			})

			When("indextemplate exists in opensearch but the name has changed", func() {
				BeforeEach(func() {
					recorder = record.NewFakeRecorder(1)

					instance.Status.IndexTemplateName = "my-template" // old template name
					instance.Spec.Name = "new-template"               // new template name
				})

				It("should fail", func() {
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
					Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s cannot change the index template name", opensearchIndexTemplateNameMismatch)))
				})
			})

			When("indextemplate doesn't exist in opensearch", func() {
				BeforeEach(func() {
					recorder = record.NewFakeRecorder(1)
					indexTemplateUrl := fmt.Sprintf("%s_index_template/my-template", clusterUrl)
					transport.RegisterResponder(
						http.MethodGet,
						indexTemplateUrl,
						httpmock.NewStringResponder(404, "does not exist").Once(failMessage),
					)
					transport.RegisterResponder(
						http.MethodPut,
						indexTemplateUrl,
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
				})

				It("should create the indextemplate", func() {
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
					Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s index template updated in opensearch", opensearchAPIUpdated)))
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
				instance.Status.ExistingIndexTemplate = pointer.Bool(true)
			})
			It("should do nothing and exit", func() {
				Expect(reconciler.Delete()).To(Succeed())
			})
		})

		Context("existing status is false", func() {
			BeforeEach(func() {
				instance.Status.ExistingIndexTemplate = pointer.Bool(false)
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

			When("indextemplate does not exist", func() {
				BeforeEach(func() {
					mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(*cluster, nil)
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
					transport.RegisterResponder(
						http.MethodHead,
						fmt.Sprintf("%s_index_template/my-template", clusterUrl),
						httpmock.NewStringResponder(404, "does not exist").Once(failMessage),
					)
				})

				It("should do nothing and exit", func() {
					Expect(reconciler.Delete()).To(Succeed())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + 1))
				})
			})

			When("indextemplate does exist", func() {
				BeforeEach(func() {
					mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(*cluster, nil)
					indexTemplateUrl := fmt.Sprintf("%s_index_template/my-template", clusterUrl)

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
					transport.RegisterResponder(
						http.MethodHead,
						indexTemplateUrl,
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
					transport.RegisterResponder(
						http.MethodDelete,
						indexTemplateUrl,
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
				})

				It("should delete the indextemplate", func() {
					Expect(reconciler.Delete()).To(Succeed())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + 1))
				})
			})
		})
	})
})
