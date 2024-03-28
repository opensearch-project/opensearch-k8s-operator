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

var _ = Describe("componenttemplate reconciler", func() {
	var (
		transport  *httpmock.MockTransport
		reconciler *ComponentTemplateReconciler
		instance   *opsterv1.OpensearchComponentTemplate
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
		instance = &opsterv1.OpensearchComponentTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-componenttemplate",
				Namespace: "test-componenttemplate",
				UID:       "testuid",
			},
			Spec: opsterv1.OpensearchComponentTemplateSpec{
				OpensearchRef: corev1.LocalObjectReference{
					Name: "test-cluster",
				},
				Name: "my-template",
				Template: opsterv1.OpensearchIndexSpec{
					Settings: &apiextensionsv1.JSON{},
					Mappings: &apiextensionsv1.JSON{},
					Aliases:  make(map[string]opsterv1.OpensearchIndexAliasSpec),
				},
				Version: 0,
				Meta:    &apiextensionsv1.JSON{},
			},
		}

		cluster = &opsterv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "test-componenttemplate",
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
		reconciler = &ComponentTemplateReconciler{
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

	When("when allow_auto_create exists", func() {
		BeforeEach(func() {
			recorder = record.NewFakeRecorder(1)
			mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(opsterv1.OpenSearchCluster{}, NotFoundError())
			instance.Spec.AllowAutoCreate = true
		})
		It("should throw a opensearchAPIUpdated event", func() {
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
			Expect(len(events)).To(Equal(2))
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
			Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s cannot change the cluster a component template refers to", opensearchRefMismatch)))
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
				instance.Status.ExistingComponentTemplate = pointer.Bool(true)
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
					fmt.Sprintf("%s_component_template/my-template", clusterUrl),
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
				instance.Status.ExistingComponentTemplate = pointer.Bool(true)
			})

			It("should do nothing", func() {
				_, err := reconciler.Reconcile()
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("existing status is false", func() {
			BeforeEach(func() {
				instance.Status.ExistingComponentTemplate = pointer.Bool(false)
			})

			When("componenttemplate exists in opensearch and is the same", func() {
				BeforeEach(func() {
					response := responses.GetComponentTemplatesResponse{
						ComponentTemplates: make([]responses.ComponentTemplate, 1),
					}
					response.ComponentTemplates[0] = responses.ComponentTemplate{
						Name: "my-template",
						ComponentTemplate: requests.ComponentTemplate{
							Template: requests.Index{
								Settings: &apiextensionsv1.JSON{},
								Mappings: &apiextensionsv1.JSON{},
								Aliases:  make(map[string]requests.IndexAlias),
							},
							Version: 0,
							Meta:    &apiextensionsv1.JSON{},
						},
					}

					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf("%s_component_template/my-template", clusterUrl),
						httpmock.NewJsonResponderOrPanic(200, response).Once(failMessage),
					)
				})

				It("should do nothing", func() {
					_, err := reconciler.Reconcile()
					Expect(err).ToNot(HaveOccurred())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
				})
			})

			When("componenttemplate exists in opensearch and is not the same", func() {
				BeforeEach(func() {
					recorder = record.NewFakeRecorder(1)

					response := responses.GetComponentTemplatesResponse{
						ComponentTemplates: make([]responses.ComponentTemplate, 1),
					}
					response.ComponentTemplates[0] = responses.ComponentTemplate{
						Name: "my-template",
						ComponentTemplate: requests.ComponentTemplate{
							Template: requests.Index{
								Settings: &apiextensionsv1.JSON{},
								Mappings: &apiextensionsv1.JSON{},
								Aliases:  make(map[string]requests.IndexAlias),
							},
							Version: 100,
							Meta:    &apiextensionsv1.JSON{},
						},
					}

					componentTemplateUrl := fmt.Sprintf("%s_component_template/my-template", clusterUrl)
					transport.RegisterResponder(
						http.MethodGet,
						componentTemplateUrl,
						httpmock.NewJsonResponderOrPanic(200, response).Once(failMessage),
					)
					transport.RegisterResponder(
						http.MethodPut,
						componentTemplateUrl,
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
				})

				It("should update the componenttemplate", func() {
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
					Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s component template updated in opensearch", opensearchAPIUpdated)))
				})
			})

			When("indextemplate exists in opensearch but the name has changed", func() {
				BeforeEach(func() {
					recorder = record.NewFakeRecorder(1)

					instance.Status.ComponentTemplateName = "my-template" // old template name
					instance.Spec.Name = "new-template"                   // new template name
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
					Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s cannot change the component template name", opensearchComponentTemplateNameMismatch)))
				})
			})

			When("componenttemplate doesn't exist in opensearch", func() {
				BeforeEach(func() {
					recorder = record.NewFakeRecorder(1)
					componentTemplateUrl := fmt.Sprintf("%s_component_template/my-template", clusterUrl)
					transport.RegisterResponder(
						http.MethodGet,
						componentTemplateUrl,
						httpmock.NewStringResponder(404, "does not exist").Once(failMessage),
					)
					transport.RegisterResponder(
						http.MethodPut,
						componentTemplateUrl,
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
				})

				It("should create the componenttemplate", func() {
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
					Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s component template updated in opensearch", opensearchAPIUpdated)))
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
				instance.Status.ExistingComponentTemplate = pointer.Bool(true)
			})
			It("should do nothing and exit", func() {
				Expect(reconciler.Delete()).To(Succeed())
			})
		})

		Context("existing status is false", func() {
			BeforeEach(func() {
				instance.Status.ExistingComponentTemplate = pointer.Bool(false)
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

			When("componenttemplate does not exist", func() {
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
						fmt.Sprintf("%s_component_template/my-template", clusterUrl),
						httpmock.NewStringResponder(404, "does not exist").Once(failMessage),
					)
				})

				It("should do nothing and exit", func() {
					Expect(reconciler.Delete()).To(Succeed())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + 1))
				})
			})

			When("componenttemplate does exist", func() {
				BeforeEach(func() {
					mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(*cluster, nil)
					componentTemplateUrl := fmt.Sprintf("%s_component_template/my-template", clusterUrl)

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
						componentTemplateUrl,
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
					transport.RegisterResponder(
						http.MethodDelete,
						componentTemplateUrl,
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
				})

				It("should delete the componenttemplate", func() {
					Expect(reconciler.Delete()).To(Succeed())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + 1))
				})
			})
		})
	})
})
