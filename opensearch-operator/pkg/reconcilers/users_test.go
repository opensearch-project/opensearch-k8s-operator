package reconcilers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/opensearch-gateway/requests"
	"opensearch.opster.io/opensearch-gateway/responses"
	"opensearch.opster.io/opensearch-gateway/services"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("users reconciler", func() {
	var (
		transport  *httpmock.MockTransport
		reconciler *UserReconciler
		instance   *opsterv1.OpensearchUser
		recorder   *record.FakeRecorder

		// Objects
		ns       *corev1.Namespace
		password *corev1.Secret
		cluster  *opsterv1.OpenSearchCluster
	)

	BeforeEach(func() {
		transport = httpmock.NewMockTransport()
		transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
		instance = &opsterv1.OpensearchUser{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-user",
				UID:  types.UID("testuid"),
			},
			Spec: opsterv1.OpensearchUserSpec{
				OpensearchRef: opsterv1.OpensearchClusterSelector{
					Name:      "test-cluster",
					Namespace: "test-user",
				},
				PasswordFrom: opsterv1.UserPasswordSpec{
					Namespace: "test-user",
					SecretKeySelector: corev1.SecretKeySelector{
						Key: "password",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test-password",
						},
					},
				},
			},
		}

		// Sleep for cache to start
		time.Sleep(time.Second)
		// Set up prereq-objects
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-user",
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
		password = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-password",
				Namespace: "test-user",
			},
			StringData: map[string]string{
				"password": "testpassword",
			},
		}
		Expect(func() error {
			err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(password), &corev1.Secret{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					return k8sClient.Create(context.Background(), password)
				}
				return err
			}
			return nil
		}()).To(Succeed())
		cluster = &opsterv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "test-user",
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
		reconciler = NewUserReconciler(
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
			Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s waiting for opensearch cluster to exist", opensearchPendingReason)))
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
			Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s waiting for opensearch cluster status to be running", opensearchPendingReason)))
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

		When("password secret is incorrect", func() {
			Context("password key does not exist", func() {
				BeforeEach(func() {
					instance.Spec.PasswordFrom.Key = "badkey"
					recorder = record.NewFakeRecorder(1)
				})
				It("should send the appropriate error", func() {
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
					Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s key badkey does not exist in secret", passwordErrorReason)))
				})
			})
			Context("secret does not exist", func() {
				BeforeEach(func() {
					instance.Spec.PasswordFrom.Name = "badsecret"
					recorder = record.NewFakeRecorder(1)
				})
				It("should send the appropriate error", func() {
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
					Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s error fetching password secret", passwordErrorReason)))
				})
			})
		})

		When("user exists with UID in opensearch", func() {
			BeforeEach(func() {
				userRequest := requests.User{
					Attributes: map[string]string{
						services.K8sAttributeField: "testuid",
					},
				}
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/internalusers/%s",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
						instance.Name,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetUserResponse{
						instance.Name: userRequest,
					}).Once(failMessage),
				)
			})

			It("should do nothing", func() {
				_, err := reconciler.Reconcile()
				Expect(err).ToNot(HaveOccurred())
				// Confirm all responders have been called
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
			})
		})
		When("user exists without UID in opensearch", func() {
			BeforeEach(func() {
				userRequest := requests.User{}
				recorder = record.NewFakeRecorder(1)
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/internalusers/%s",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
						instance.Name,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetUserResponse{
						instance.Name: userRequest,
					}).Once(failMessage),
				)
			})
			It("should error", func() {
				go func() {
					defer GinkgoRecover()
					defer close(recorder.Events)
					_, err := reconciler.Reconcile()
					Expect(err).To(HaveOccurred())
					// Confirm all responders have been called
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
				}()
				var events []string
				for msg := range recorder.Events {
					events = append(events, msg)
				}
				Expect(len(events)).To(Equal(1))
				Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s failed to get user status from Opensearch API", opensearchAPIError)))
			})
		})
		When("user exists and is different", func() {
			BeforeEach(func() {
				userRequest := requests.User{
					Attributes: map[string]string{
						services.K8sAttributeField: "testuid",
					},
				}
				recorder = record.NewFakeRecorder(1)
				instance.Spec.BackendRoles = []string{
					"testbackend",
				}
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/internalusers/%s",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
						instance.Name,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetUserResponse{
						instance.Name: userRequest,
					}).Once(failMessage),
				)
				transport.RegisterResponder(
					http.MethodPut,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/internalusers/%s",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
						instance.Name,
					),
					httpmock.NewStringResponder(200, "OK").Once(failMessage),
				)
			})
			It("should update the user", func() {
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
				Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s user updated in opensearch", opensearchAPIUpdated)))
			})
		})
		When("user does not exist", func() {
			BeforeEach(func() {
				recorder = record.NewFakeRecorder(1)
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/internalusers/%s",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
						instance.Name,
					),
					httpmock.NewStringResponder(404, "does not exist").Once(failMessage),
				)
				transport.RegisterResponder(
					http.MethodPut,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/internalusers/%s",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
						instance.Name,
					),
					httpmock.NewStringResponder(200, "OK").Once(failMessage),
				)
			})
			It("should create the user", func() {
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
				Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s user updated in opensearch", opensearchAPIUpdated)))
			})
		})
	})
	Context("deletions", func() {
		extraContextCalls := 1

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
		})

		When("the opensearch cluster does not exist", func() {
			BeforeEach(func() {
				instance.Spec.OpensearchRef.Name = "doesnotexist"
			})
			It("should do nothing", func() {
				Expect(reconciler.Delete()).To(Succeed())
			})
		})
		When("the user does not exist", func() {
			BeforeEach(func() {
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/internalusers/%s",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
						instance.Name,
					),
					httpmock.NewStringResponder(404, "does not exist").Once(failMessage),
				)
			})
			It("should do nothing", func() {
				Expect(reconciler.Delete()).To(Succeed())
				// Confirm all responders have been called
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
			})
		})
		When("the user exists without a UID", func() {
			BeforeEach(func() {
				userRequest := requests.User{}
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/internalusers/%s",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
						instance.Name,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetUserResponse{
						instance.Name: userRequest,
					}).Times(2, failMessage),
				)
			})
			It("should do nothing", func() {
				Expect(reconciler.Delete()).To(Succeed())
				// Confirm all responders have been called
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
			})
		})
		When("the user exists with correct UID", func() {
			BeforeEach(func() {
				userRequest := requests.User{
					Attributes: map[string]string{
						services.K8sAttributeField: "testuid",
					},
				}
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/internalusers/%s",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
						instance.Name,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetUserResponse{
						instance.Name: userRequest,
					}).Times(2, failMessage),
				)
				transport.RegisterResponder(
					http.MethodDelete,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/internalusers/%s",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
						instance.Name,
					),
					httpmock.NewStringResponder(200, "OK").Once(failMessage),
				)
			})
			It("should delete the user", func() {
				Expect(reconciler.Delete()).To(Succeed())
				// Confirm all responders have been called
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
			})
		})
	})
})
