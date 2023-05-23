package reconcilers

import (
	"context"
	"fmt"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"net/http"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/opensearch-gateway/requests"
	"opensearch.opster.io/opensearch-gateway/responses"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var _ = Describe("tenant reconciler", func() {
	var (
		transport  *httpmock.MockTransport
		reconciler *TenantReconciler
		instance   *opsterv1.OpensearchTenant
		recorder   *record.FakeRecorder

		// Objects
		ns      *corev1.Namespace
		cluster *opsterv1.OpenSearchCluster
	)

	BeforeEach(func() {
		transport = httpmock.NewMockTransport()
		transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
		instance = &opsterv1.OpensearchTenant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tenant",
				Namespace: "test-tenant",
				UID:       "testuid",
			},
			Spec: opsterv1.OpensearchTenantSpec{
				OpensearchRef: corev1.LocalObjectReference{
					Name: "test-cluster",
				},
				Description: "test-description",
			},
		}

		// Sleep for cache to start
		time.Sleep(time.Second)
		// Set up prereq-objects
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-tenant",
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
				Namespace: "test-tenant",
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
		reconciler = NewTenantReconciler(
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
			Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s cannot change the cluster an tenant refers to", opensearchRefMismatch)))
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
				instance.Status.ExistingTenant = pointer.Bool(true)
			})

			It("should do nothing", func() {
				_, err := reconciler.Reconcile()
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("existing status is nil", func() {
			var localExtraCalls = 4
			BeforeEach(func() {
				tenantRequest := requests.Tenant{
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
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/tenants/%s",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
						instance.Name,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetTenantResponse{
						instance.Name: tenantRequest,
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
				instance.Status.ExistingTenant = pointer.Bool(true)
			})
			It("should do nothing", func() {
				_, err := reconciler.Reconcile()
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("existing status is false", func() {
			BeforeEach(func() {
				instance.Status.ExistingTenant = pointer.Bool(false)
			})

			When("tenant exists in opensearch and is the same", func() {
				BeforeEach(func() {
					tenantRequest := requests.Tenant{
						Description: "test-description",
					}
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/tenants/%s",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
							instance.Name,
						),
						httpmock.NewJsonResponderOrPanic(200, responses.GetTenantResponse{
							instance.Name: tenantRequest,
						}).Once(failMessage),
					)
				})
				It("should do nothing", func() {
					_, err := reconciler.Reconcile()
					Expect(err).ToNot(HaveOccurred())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
				})
			})
			When("tenant exists in opensearch and is not the same", func() {
				BeforeEach(func() {
					recorder = record.NewFakeRecorder(1)
					tenantRequest := requests.Tenant{
						Description: "some-other-description",
					}
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/tenants/%s",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
							instance.Name,
						),
						httpmock.NewJsonResponderOrPanic(200, responses.GetTenantResponse{
							instance.Name: tenantRequest,
						}).Once(failMessage),
					)
					transport.RegisterResponder(
						http.MethodPut,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/tenants/%s",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
							instance.Name,
						),
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
				})
				It("should update the tenant", func() {
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
					Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s tenant updated in opensearch", opensearchAPIUpdated)))
				})
			})
			When("tenant doesn't exist in opensearch", func() {
				BeforeEach(func() {
					recorder = record.NewFakeRecorder(1)
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/tenants/%s",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
							instance.Name,
						),
						httpmock.NewStringResponder(404, "does not exist").Once(failMessage),
					)
					transport.RegisterResponder(
						http.MethodPut,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/tenants/%s",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
							instance.Name,
						),
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
				})
				It("should create the tenant", func() {
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
					Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s tenant updated in opensearch", opensearchAPIUpdated)))
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
				instance.Status.ExistingTenant = pointer.Bool(true)
			})
			It("should do nothing and exit", func() {
				Expect(reconciler.Delete()).To(Succeed())
			})
		})

		Context("existing status is false", func() {
			BeforeEach(func() {
				instance.Status.ExistingTenant = pointer.Bool(false)
			})

			When("cluster does not exist", func() {
				BeforeEach(func() {
					instance.Spec.OpensearchRef.Name = "doesnotexist"
				})
				It("should do nothing and exit", func() {
					Expect(reconciler.Delete()).To(Succeed())
				})
			})

			When("tenant does not exist", func() {
				BeforeEach(func() {
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
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/tenants/%s",
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
			When("tenant does exist", func() {
				BeforeEach(func() {
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
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/tenants/%s",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
							instance.Name,
						),
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
					transport.RegisterResponder(
						http.MethodDelete,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/tenants/%s",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
							instance.Name,
						),
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
				})
				It("should delete the tenant", func() {
					Expect(reconciler.Delete()).To(Succeed())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + 1))
				})
			})
		})
	})
})
