package reconcilers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/opensearch-gateway/requests"
	"opensearch.opster.io/opensearch-gateway/responses"
	"opensearch.opster.io/pkg/reconcilers/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("indextemplate reconciler", func() {
	var (
		transport  *httpmock.MockTransport
		reconciler *IndexTemplateReconciler
		instance   *opsterv1.OpensearchIndexTemplate
		recorder   *record.FakeRecorder

		// Objects
		ns         *corev1.Namespace
		cluster    *opsterv1.OpenSearchCluster
		clusterUrl string
	)

	BeforeEach(func() {
		transport = httpmock.NewMockTransport()
		util.GetTransport = func(ctx context.Context, k8sClient client.Client, cluster *opsterv1.OpenSearchCluster) (http.RoundTripper, error) {
			return transport, nil
		}
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

		// Sleep for cache to start
		time.Sleep(time.Millisecond * 100)
		// Set up prereq-objects
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-indextemplate",
			},
		}
		Expect(func() error {
			err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(ns), &corev1.Namespace{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					return k8sClient.Create(context.Background(), ns)
				}
				return err
			}
			return nil
		}()).To(Succeed())

		cluster = &opsterv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "test-indextemplate",
			},
			Spec: opsterv1.ClusterSpec{
				General: opsterv1.GeneralConfig{
					ServiceName: "test-cluster",
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

		Expect(func() error {
			err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(cluster), &opsterv1.OpenSearchCluster{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					return k8sClient.Create(context.Background(), cluster)
				}
				return err
			}
			return nil
		}()).To(Succeed())
	})

	JustBeforeEach(func() {
		reconciler = NewIndexTemplateReconciler(
			context.Background(),
			k8sClient,
			recorder,
			instance,
			WithOSClientTransport(transport),
			WithUpdateStatus(false),
		)
	})

	When("cluster doesn't exist", func() {
		BeforeEach(func() {
			instance.Spec.OpensearchRef.Name = "doesnotexist"
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
			Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(cluster), cluster)).To(Succeed())
			cluster.Status.Phase = opsterv1.PhaseRunning
			cluster.Status.ComponentsStatus = []opsterv1.ComponentStatus{}
			Expect(k8sClient.Status().Update(context.Background(), cluster)).To(Succeed())
			Eventually(func() string {
				err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(cluster), cluster)
				if err != nil {
					return "failed"
				}
				return cluster.Status.Phase
			}).Should(Equal(opsterv1.PhaseRunning))

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
			var localExtraCalls = 4
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
				})
				It("should do nothing and exit", func() {
					Expect(reconciler.Delete()).To(Succeed())
				})
			})

			When("indextemplate does not exist", func() {
				BeforeEach(func() {
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
