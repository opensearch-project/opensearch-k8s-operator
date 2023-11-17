package reconcilers

import (
	"context"
	"fmt"
	"net/http"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/mocks/github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("users reconciler", func() {
	var (
		transport  *httpmock.MockTransport
		reconciler *UserReconciler
		instance   *opsterv1.OpensearchUser
		recorder   *record.FakeRecorder
		mockClient *k8s.MockK8sClient

		// Objects
		password *corev1.Secret
		cluster  *opsterv1.OpenSearchCluster
	)

	BeforeEach(func() {
		mockClient = k8s.NewMockK8sClient(GinkgoT())
		transport = httpmock.NewMockTransport()
		transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
		instance = &opsterv1.OpensearchUser{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-user",
				Namespace: "test-user",
				UID:       types.UID("testuid"),
			},
			Spec: opsterv1.OpensearchUserSpec{
				OpensearchRef: corev1.LocalObjectReference{
					Name: "test-cluster",
				},
				PasswordFrom: corev1.SecretKeySelector{
					Key: "password",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-password",
					},
				},
			},
		}
		password = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-password",
				Namespace: "test-user",
			},
			Data: map[string][]byte{
				"password": []byte("testpassword"),
			},
		}
		cluster = &opsterv1.OpenSearchCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "test-user",
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
		reconciler = &UserReconciler{
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

		When("password secret is incorrect", func() {
			Context("password key does not exist", func() {
				BeforeEach(func() {
					instance.Spec.PasswordFrom.Key = "badkey"
					mockClient.EXPECT().GetSecret(mock.Anything, mock.Anything).Return(*password, nil)
					recorder = record.NewFakeRecorder(1)
				})
				It("should send the appropriate error", func() {
					var createdSecret *corev1.Secret
					mockClient.On("CreateSecret", mock.Anything).
						Return(func(secret *corev1.Secret) (*ctrl.Result, error) {
							createdSecret = secret
							return &ctrl.Result{}, nil
						})
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
					Expect(createdSecret).ToNot(BeNil())
					Expect(len(events)).To(Equal(1))
					Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s key badkey does not exist in secret", passwordError)))
				})
			})
			Context("secret does not exist", func() {
				BeforeEach(func() {
					instance.Spec.PasswordFrom.Name = "badsecret"
					mockClient.EXPECT().GetSecret(mock.Anything, mock.Anything).Return(corev1.Secret{}, NotFoundError())
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
					Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s error fetching password secret", passwordError)))
				})
			})
		})

		When("user exists with UID in opensearch", func() {
			BeforeEach(func() {
				mockClient.EXPECT().GetSecret(mock.Anything, mock.Anything).Return(*password, nil)
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
				recorder = record.NewFakeRecorder(1)
			})

			It("should do nothing", func() {
				var createdSecret *corev1.Secret
				mockClient.On("CreateSecret", mock.Anything).
					Return(func(secret *corev1.Secret) (*ctrl.Result, error) {
						createdSecret = secret
						return &ctrl.Result{}, nil
					})
				defer close(recorder.Events)
				_, err := reconciler.Reconcile()
				Expect(err).ToNot(HaveOccurred())
				// Confirm all responders have been called
				Expect(createdSecret).ToNot(BeNil())
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
			})
		})
		When("user exists without UID in opensearch", func() {
			BeforeEach(func() {
				mockClient.EXPECT().GetSecret(mock.Anything, mock.Anything).Return(*password, nil)
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
				var createdSecret *corev1.Secret
				mockClient.On("CreateSecret", mock.Anything).
					Return(func(secret *corev1.Secret) (*ctrl.Result, error) {
						createdSecret = secret
						return &ctrl.Result{}, nil
					})
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
				Expect(createdSecret).ToNot(BeNil())
				Expect(len(events)).To(Equal(1))
				Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s failed to get user status from Opensearch API", opensearchAPIError)))
			})
		})
		When("user exists and is different", func() {
			BeforeEach(func() {
				mockClient.EXPECT().GetSecret(mock.Anything, mock.Anything).Return(*password, nil)
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
				var createdSecret *corev1.Secret
				mockClient.On("CreateSecret", mock.Anything).
					Return(func(secret *corev1.Secret) (*ctrl.Result, error) {
						createdSecret = secret
						return &ctrl.Result{}, nil
					})
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
				Expect(createdSecret).ToNot(BeNil())
				Expect(len(events)).To(Equal(1))
				Expect(events[0]).To(Equal(fmt.Sprintf("Normal %s user updated in opensearch", opensearchAPIUpdated)))
			})
			It("should update the secret with opensearch annotations", func() {
				var createdSecret *corev1.Secret
				mockClient.On("CreateSecret", mock.Anything).
					Return(func(secret *corev1.Secret) (*ctrl.Result, error) {
						createdSecret = secret
						return &ctrl.Result{}, nil
					})
				defer close(recorder.Events)
				_, err := reconciler.Reconcile()
				Expect(err).ToNot(HaveOccurred())

				annotations := createdSecret.GetAnnotations()

				actualName := annotations[helpers.OsUserNameAnnotation]
				actualNamespace := annotations[helpers.OsUserNamespaceAnnotation]

				expectedName := "test-user"
				expectedNamespace := "test-user"
				Expect(actualName).To(Equal(expectedName))
				Expect(actualNamespace).To(Equal(expectedNamespace))
			})
		})
		When("user does not exist", func() {
			BeforeEach(func() {
				mockClient.EXPECT().GetSecret(mock.Anything, mock.Anything).Return(*password, nil)
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
				var createdSecret *corev1.Secret
				mockClient.On("CreateSecret", mock.Anything).
					Return(func(secret *corev1.Secret) (*ctrl.Result, error) {
						createdSecret = secret
						return &ctrl.Result{}, nil
					})
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
				Expect(createdSecret).ToNot(BeNil())
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
				mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(opsterv1.OpenSearchCluster{}, NotFoundError())
			})
			It("should do nothing", func() {
				Expect(reconciler.Delete()).To(Succeed())
			})
		})
		When("the user does not exist", func() {
			BeforeEach(func() {
				mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(*cluster, nil)
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
				mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(*cluster, nil)
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
				mockClient.EXPECT().GetOpenSearchCluster(mock.Anything, mock.Anything).Return(*cluster, nil)
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
