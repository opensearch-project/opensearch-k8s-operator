package reconcilers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/opensearch-gateway/requests"
	"opensearch.opster.io/opensearch-gateway/responses"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("userrolebinding reconciler", func() {
	var (
		transport  *httpmock.MockTransport
		reconciler *UserRoleBindingReconciler
		instance   *opsterv1.OpensearchUserRoleBinding
		recorder   *record.FakeRecorder

		// Objects
		ns      *corev1.Namespace
		cluster *opsterv1.OpenSearchCluster
	)

	BeforeEach(func() {
		transport = httpmock.NewMockTransport()
		transport.RegisterNoResponder(httpmock.NewNotFoundResponder(failMessage))
		instance = &opsterv1.OpensearchUserRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-role",
				Namespace: "test-urb",
				UID:       types.UID("testuid"),
			},
			Spec: opsterv1.OpensearchUserRoleBindingSpec{
				OpensearchRef: corev1.LocalObjectReference{
					Name: "test-cluster",
				},
				Users: []string{
					"test-user",
				},
				Roles: []string{
					"test-role",
				},
				BackendRoles: []string{
					"test-backend-role",
				},
			},
		}

		// Sleep for cache to start
		time.Sleep(time.Second)
		// Set up prereq-objects
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-urb",
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
				Namespace: "test-urb",
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
		reconciler = NewUserRoleBindingReconciler(
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
			Expect(events[0]).To(Equal(fmt.Sprintf("Warning %s cannot change the cluster a userrolebinding refers to", opensearchRefMismatch)))
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

		When("role mapping does not exist", func() {
			var users []string
			var backendRoles []string
			BeforeEach(func() {
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					httpmock.NewStringResponder(404, "does not exist").Once(failMessage),
				)
				transport.RegisterResponder(
					http.MethodPut,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					func(req *http.Request) (*http.Response, error) {
						mapping := &requests.RoleMapping{}
						if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
							return httpmock.NewStringResponse(501, ""), nil
						}
						users = mapping.Users
						backendRoles = mapping.BackendRoles
						return httpmock.NewStringResponse(200, ""), nil
					},
				)
			})
			It("should create the role mapping", func() {
				_, err := reconciler.Reconcile()
				Expect(err).NotTo(HaveOccurred())
				Expect(users).To(ContainElement("test-user"))
				Expect(backendRoles).To(ContainElement("test-backend-role"))
				// Confirm all responders have been called
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
			})
		})
		When("role mapping exists, user and backend role are in the list", func() {
			BeforeEach(func() {
				roleMappingRequest := requests.RoleMapping{
					Users: []string{
						"someother-user",
						"test-user",
					},
					BackendRoles: []string{
						"test-backend-role",
						"someother-backend-role",
					},
				}
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
						"test-role": roleMappingRequest,
					}).Times(2, failMessage),
				)
			})
			It("should do nothing", func() {
				_, err := reconciler.Reconcile()
				Expect(err).NotTo(HaveOccurred())
				// Confirm all responders have been called
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
			})
		})
		When("role mapping exists, user and backendRole is not in the list", func() {
			var users []string
			var backendRoles []string
			BeforeEach(func() {
				roleMappingRequest := requests.RoleMapping{
					Users: []string{
						"someother-user",
					},
					BackendRoles: []string{
						"someother-backend-role",
					},
				}
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
						"test-role": roleMappingRequest,
					}).Times(2, failMessage),
				)
				transport.RegisterResponder(
					http.MethodPut,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					func(req *http.Request) (*http.Response, error) {
						mapping := &requests.RoleMapping{}
						if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
							return httpmock.NewStringResponse(501, ""), nil
						}
						users = mapping.Users
						backendRoles = mapping.BackendRoles
						return httpmock.NewStringResponse(200, ""), nil
					},
				)
			})
			It("should update the mapping", func() {
				_, err := reconciler.Reconcile()
				Expect(err).NotTo(HaveOccurred())
				Expect(users).To(ContainElements("test-user", "someother-user"))
				Expect(backendRoles).To(ContainElements("test-backend-role", "someother-backend-role"))
				// Confirm all responders have been called
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
			})
		})

		When("role mapping exists, and user only is not in the list", func() {
			var users []string
			var backendRoles []string
			BeforeEach(func() {
				roleMappingRequest := requests.RoleMapping{
					Users: []string{
						"someother-user",
					},
					BackendRoles: []string{
						"test-backend-role",
						"someother-backend-role",
					},
				}
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
						"test-role": roleMappingRequest,
					}).Times(2, failMessage),
				)
				transport.RegisterResponder(
					http.MethodPut,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					func(req *http.Request) (*http.Response, error) {
						mapping := &requests.RoleMapping{}
						if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
							return httpmock.NewStringResponse(501, ""), nil
						}
						users = mapping.Users
						backendRoles = mapping.BackendRoles
						return httpmock.NewStringResponse(200, ""), nil
					},
				)
			})
			It("should update the mapping", func() {
				_, err := reconciler.Reconcile()
				Expect(err).NotTo(HaveOccurred())
				Expect(users).To(ContainElements("test-user", "someother-user"))
				Expect(backendRoles).To(ContainElements("test-backend-role", "someother-backend-role"))
				// Confirm all responders have been called
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
			})
		})

		When("role mapping exists, and backendRole only is not in the list", func() {
			var users []string
			var backendRoles []string
			BeforeEach(func() {
				roleMappingRequest := requests.RoleMapping{
					Users: []string{
						"test-user",
						"someother-user",
					},
					BackendRoles: []string{
						"someother-backend-role",
					},
				}
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
						"test-role": roleMappingRequest,
					}).Times(2, failMessage),
				)
				transport.RegisterResponder(
					http.MethodPut,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					func(req *http.Request) (*http.Response, error) {
						mapping := &requests.RoleMapping{}
						if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
							return httpmock.NewStringResponse(501, ""), nil
						}
						users = mapping.Users
						backendRoles = mapping.BackendRoles
						return httpmock.NewStringResponse(200, ""), nil
					},
				)
			})
			It("should update the mapping", func() {
				_, err := reconciler.Reconcile()
				Expect(err).NotTo(HaveOccurred())
				Expect(users).To(ContainElements("test-user", "someother-user"))
				Expect(backendRoles).To(ContainElements("test-backend-role", "someother-backend-role"))
				// Confirm all responders have been called
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
			})
		})

		When("user and backendRole has been removed from the binding", func() {
			var users []string
			var backendRoles []string
			BeforeEach(func() {
				instance.Status.ProvisionedRoles = []string{
					"test-role",
				}
				instance.Status.ProvisionedUsers = []string{
					"someother-user",
					"test-user",
				}
				instance.Status.ProvisionedBackendRoles = []string{
					"test-backend-role",
					"someother-backend-role",
				}
				roleMappingRequest := requests.RoleMapping{
					Users: []string{
						"someother-user",
						"test-user",
						"another-user",
					},
					BackendRoles: []string{
						"someother-backend-role",
						"test-backend-role",
						"another-backend-role",
					},
				}
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
						"test-role": roleMappingRequest,
					}).Times(3, failMessage),
				)
				transport.RegisterResponder(
					http.MethodPut,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					func(req *http.Request) (*http.Response, error) {
						mapping := &requests.RoleMapping{}
						if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
							return httpmock.NewStringResponse(501, ""), nil
						}
						users = mapping.Users
						backendRoles = mapping.BackendRoles
						return httpmock.NewStringResponse(200, ""), nil
					},
				)
			})
			It("should remove the user and the backendRole from the role", func() {
				_, err := reconciler.Reconcile()
				Expect(err).NotTo(HaveOccurred())
				Expect(users).To(ContainElements("test-user", "another-user"))
				Expect(users).NotTo(ContainElement("someother-user"))
				Expect(backendRoles).To(ContainElements("test-backend-role", "another-backend-role"))
				Expect(backendRoles).NotTo(ContainElement("someother-backend-role"))
				// Confirm all responders have been called
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
			})
		})

		When("user only has been removed from the binding", func() {
			var users []string
			var backendRoles []string
			BeforeEach(func() {
				instance.Spec.BackendRoles = []string{
					"test-backend-role",
					"someother-backend-role",
				}
				instance.Status.ProvisionedRoles = []string{
					"test-role",
				}
				instance.Status.ProvisionedUsers = []string{
					"someother-user",
					"test-user",
				}
				instance.Status.ProvisionedBackendRoles = []string{
					"test-backend-role",
					"someother-backend-role",
				}
				roleMappingRequest := requests.RoleMapping{
					Users: []string{
						"someother-user",
						"test-user",
						"another-user",
					},
					BackendRoles: []string{
						"someother-backend-role",
						"test-backend-role",
						"another-backend-role",
					},
				}
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
						"test-role": roleMappingRequest,
					}).Times(3, failMessage),
				)
				transport.RegisterResponder(
					http.MethodPut,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					func(req *http.Request) (*http.Response, error) {
						mapping := &requests.RoleMapping{}
						if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
							return httpmock.NewStringResponse(501, ""), nil
						}
						users = mapping.Users
						backendRoles = mapping.BackendRoles
						return httpmock.NewStringResponse(200, ""), nil
					},
				)
			})
			It("should remove the user from the role", func() {
				_, err := reconciler.Reconcile()
				Expect(err).NotTo(HaveOccurred())
				Expect(users).To(ContainElements("test-user", "another-user"))
				Expect(users).NotTo(ContainElement("someother-user"))
				Expect(backendRoles).To(ContainElements("test-backend-role", "another-backend-role", "someother-backend-role"))
				// Confirm all responders have been called
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
			})
		})

		When("backendRole only has been removed from the binding", func() {
			var users []string
			var backendRoles []string
			BeforeEach(func() {
				instance.Spec.Users = []string{
					"someother-user",
					"test-user",
				}
				instance.Status.ProvisionedRoles = []string{
					"test-role",
				}
				instance.Status.ProvisionedUsers = []string{
					"someother-user",
					"test-user",
				}
				instance.Status.ProvisionedBackendRoles = []string{
					"test-backend-role",
					"someother-backend-role",
				}
				roleMappingRequest := requests.RoleMapping{
					Users: []string{
						"someother-user",
						"test-user",
						"another-user",
					},
					BackendRoles: []string{
						"someother-backend-role",
						"test-backend-role",
						"another-backend-role",
					},
				}
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
						"test-role": roleMappingRequest,
					}).Times(3, failMessage),
				)
				transport.RegisterResponder(
					http.MethodPut,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					func(req *http.Request) (*http.Response, error) {
						mapping := &requests.RoleMapping{}
						if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
							return httpmock.NewStringResponse(501, ""), nil
						}
						users = mapping.Users
						backendRoles = mapping.BackendRoles
						return httpmock.NewStringResponse(200, ""), nil
					},
				)
			})
			It("should remove the backendRole from the role", func() {
				_, err := reconciler.Reconcile()
				Expect(err).NotTo(HaveOccurred())
				Expect(users).To(ContainElements("test-user", "another-user", "someother-user"))
				Expect(backendRoles).To(ContainElements("test-backend-role", "another-backend-role"))
				Expect(backendRoles).NotTo(ContainElement("someother-backend-role"))
				// Confirm all responders have been called
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
			})
		})

		When("A role has been removed from the binding. Binding has user and backendRole", func() {
			var users []string
			var backendRoles []string
			BeforeEach(func() {
				instance.Status.ProvisionedRoles = []string{
					"test-role",
					"another-role",
				}
				instance.Status.ProvisionedUsers = []string{
					"test-user",
				}
				instance.Status.ProvisionedBackendRoles = []string{
					"test-backend-role",
				}
				roleMappingRequest := requests.RoleMapping{
					Users: []string{
						"someother-user",
						"test-user",
					},
					BackendRoles: []string{
						"someother-backend-role",
						"test-backend-role",
					},
				}
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
						"test-role": roleMappingRequest,
					}).Times(2, failMessage),
				)
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/another-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
						"another-role": roleMappingRequest,
					}).Times(2, failMessage),
				)
				transport.RegisterResponder(
					http.MethodPut,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/another-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					func(req *http.Request) (*http.Response, error) {
						mapping := &requests.RoleMapping{}
						if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
							return httpmock.NewStringResponse(501, ""), nil
						}
						users = mapping.Users
						backendRoles = mapping.BackendRoles
						return httpmock.NewStringResponse(200, ""), nil
					},
				)
			})
			It("should remove the user and the backendRole from the removed role", func() {
				_, err := reconciler.Reconcile()
				Expect(err).NotTo(HaveOccurred())
				Expect(users).To(ContainElement("someother-user"))
				Expect(users).NotTo(ContainElement("test-user"))
				Expect(backendRoles).To(ContainElement("someother-backend-role"))
				Expect(backendRoles).NotTo(ContainElement("test-backend-role"))
				// Confirm all responders have been called
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 2))
			})
		})

		When("A role has been removed from the binding. Binding has user only", func() {
			var users []string
			var backendRoles []string
			BeforeEach(func() {
				instance.Spec.BackendRoles = []string{}
				instance.Status.ProvisionedRoles = []string{
					"test-role",
					"another-role",
				}
				instance.Status.ProvisionedUsers = []string{
					"test-user",
				}
				roleMappingRequest := requests.RoleMapping{
					Users: []string{
						"someother-user",
						"test-user",
					},
				}
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
						"test-role": roleMappingRequest,
					}).Times(2, failMessage),
				)
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/another-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
						"another-role": roleMappingRequest,
					}).Times(2, failMessage),
				)
				transport.RegisterResponder(
					http.MethodPut,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/another-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					func(req *http.Request) (*http.Response, error) {
						mapping := &requests.RoleMapping{}
						if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
							return httpmock.NewStringResponse(501, ""), nil
						}
						users = mapping.Users
						backendRoles = mapping.BackendRoles
						return httpmock.NewStringResponse(200, ""), nil
					},
				)
			})
			It("should remove the user from the removed role", func() {
				_, err := reconciler.Reconcile()
				Expect(err).NotTo(HaveOccurred())
				Expect(users).To(ContainElement("someother-user"))
				Expect(users).NotTo(ContainElement("test-user"))
				Expect(backendRoles).To(BeEmpty())
				// Confirm all responders have been called
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 2))
			})
		})

		When("A role has been removed from the binding. Binding has backendRole only", func() {
			var users []string
			var backendRoles []string
			BeforeEach(func() {
				instance.Spec.Users = []string{}
				instance.Status.ProvisionedRoles = []string{
					"test-role",
					"another-role",
				}
				instance.Status.ProvisionedBackendRoles = []string{
					"test-backend-role",
				}
				roleMappingRequest := requests.RoleMapping{
					BackendRoles: []string{
						"someother-backend-role",
						"test-backend-role",
					},
				}
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
						"test-role": roleMappingRequest,
					}).Times(2, failMessage),
				)
				transport.RegisterResponder(
					http.MethodGet,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/another-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
						"another-role": roleMappingRequest,
					}).Times(2, failMessage),
				)
				transport.RegisterResponder(
					http.MethodPut,
					fmt.Sprintf(
						"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/another-role",
						cluster.Spec.General.ServiceName,
						cluster.Namespace,
					),
					func(req *http.Request) (*http.Response, error) {
						mapping := &requests.RoleMapping{}
						if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
							return httpmock.NewStringResponse(501, ""), nil
						}
						users = mapping.Users
						backendRoles = mapping.BackendRoles
						return httpmock.NewStringResponse(200, ""), nil
					},
				)
			})
			It("should remove the backendRole from the removed role", func() {
				_, err := reconciler.Reconcile()
				Expect(err).NotTo(HaveOccurred())
				Expect(users).To(BeEmpty())
				Expect(backendRoles).To(ContainElement("someother-backend-role"))
				Expect(backendRoles).NotTo(ContainElement("test-backend-role"))
				// Confirm all responders have been called
				Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 2))
			})
		})
	})

	Context("deletions", func() {
		When("cluster does not exist", func() {
			BeforeEach(func() {
				instance.Spec.OpensearchRef.Name = "doesnotexist"
			})
			It("should do nothing and exit", func() {
				Expect(reconciler.Delete()).To(Succeed())
			})
		})
		Context("checking mappings", func() {
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

			When("role mapping does not exist", func() {
				BeforeEach(func() {
					instance.Status.ProvisionedRoles = []string{
						"test-role",
					}
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						httpmock.NewStringResponder(404, "does not exist").Once(failMessage),
					)
				})
				It("should do nothing and exit", func() {
					Expect(reconciler.Delete()).To(Succeed())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls))
				})
			})

			When("user is only user and backendRole is only backendRole in role mapping", func() {
				BeforeEach(func() {
					instance.Status.ProvisionedRoles = []string{
						"test-role",
					}
					instance.Status.ProvisionedUsers = []string{
						"test-user",
					}
					instance.Status.ProvisionedBackendRoles = []string{
						"test-backend-role",
					}
					roleMappingRequest := requests.RoleMapping{
						Users: []string{
							"test-user",
						},
						BackendRoles: []string{
							"test-backend-role",
						},
					}
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
							"test-role": roleMappingRequest,
						}).Times(2, failMessage),
					)
					transport.RegisterResponder(
						http.MethodDelete,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
				})
				It("should delete the role mapping", func() {
					Expect(reconciler.Delete()).To(Succeed())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
				})
			})
			When("user is only user in role mapping", func() {
				BeforeEach(func() {
					instance.Status.ProvisionedRoles = []string{
						"test-role",
					}
					instance.Status.ProvisionedUsers = []string{
						"test-user",
					}
					roleMappingRequest := requests.RoleMapping{
						Users: []string{
							"test-user",
						},
					}
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
							"test-role": roleMappingRequest,
						}).Times(2, failMessage),
					)
					transport.RegisterResponder(
						http.MethodDelete,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
				})
				It("should delete the role mapping", func() {
					Expect(reconciler.Delete()).To(Succeed())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
				})
			})
			When("backendRole is only backendRole in role mapping", func() {
				BeforeEach(func() {
					instance.Status.ProvisionedRoles = []string{
						"test-role",
					}
					instance.Status.ProvisionedBackendRoles = []string{
						"test-backend-role",
					}
					roleMappingRequest := requests.RoleMapping{
						BackendRoles: []string{
							"test-backend-role",
						},
					}
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
							"test-role": roleMappingRequest,
						}).Times(2, failMessage),
					)
					transport.RegisterResponder(
						http.MethodDelete,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						httpmock.NewStringResponder(200, "OK").Once(failMessage),
					)
				})
				It("should delete the role mapping", func() {
					Expect(reconciler.Delete()).To(Succeed())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
				})
			})
			When("user and backendRole are one of the users and backendRoles in the mapping", func() {
				var users []string
				var backendRoles []string
				BeforeEach(func() {
					instance.Status.ProvisionedRoles = []string{
						"test-role",
					}
					instance.Status.ProvisionedUsers = []string{
						"test-user",
					}
					instance.Status.ProvisionedBackendRoles = []string{
						"test-backend-role",
					}
					roleMappingRequest := requests.RoleMapping{
						Users: []string{
							"someother-user",
							"test-user",
						},
						BackendRoles: []string{
							"someother-backend-role",
							"test-backend-role",
						},
					}
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
							"test-role": roleMappingRequest,
						}).Times(2, failMessage),
					)
					transport.RegisterResponder(
						http.MethodPut,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						func(req *http.Request) (*http.Response, error) {
							mapping := &requests.RoleMapping{}
							if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
								return httpmock.NewStringResponse(501, ""), nil
							}
							users = mapping.Users
							backendRoles = mapping.BackendRoles
							return httpmock.NewStringResponse(200, ""), nil
						},
					)
				})
				It("should remove the user and the backendRole and update the mapping", func() {
					Expect(reconciler.Delete()).To(Succeed())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
					Expect(users).To(ContainElement("someother-user"))
					Expect(users).NotTo(ContainElement("test-user"))
					Expect(backendRoles).To(ContainElement("someother-backend-role"))
					Expect(backendRoles).NotTo(ContainElement("test-backend-role"))
				})
			})
			When("user is one of the users in the mapping", func() {
				var users []string
				var backendRoles []string
				BeforeEach(func() {
					instance.Status.ProvisionedRoles = []string{
						"test-role",
					}
					instance.Status.ProvisionedUsers = []string{
						"test-user",
					}
					roleMappingRequest := requests.RoleMapping{
						Users: []string{
							"someother-user",
							"test-user",
						},
					}
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
							"test-role": roleMappingRequest,
						}).Times(2, failMessage),
					)
					transport.RegisterResponder(
						http.MethodPut,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						func(req *http.Request) (*http.Response, error) {
							mapping := &requests.RoleMapping{}
							if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
								return httpmock.NewStringResponse(501, ""), nil
							}
							users = mapping.Users
							backendRoles = mapping.BackendRoles
							return httpmock.NewStringResponse(200, ""), nil
						},
					)
				})
				It("should remove the user and update the mapping", func() {
					Expect(reconciler.Delete()).To(Succeed())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
					Expect(users).To(ContainElement("someother-user"))
					Expect(users).NotTo(ContainElement("test-user"))
					Expect(backendRoles).To(BeEmpty())
				})
			})
			When("backendRole is one of the backendRole in the mapping", func() {
				var users []string
				var backendRoles []string
				BeforeEach(func() {
					instance.Status.ProvisionedRoles = []string{
						"test-role",
					}
					instance.Status.ProvisionedBackendRoles = []string{
						"test-backend-role",
					}
					roleMappingRequest := requests.RoleMapping{
						BackendRoles: []string{
							"someother-backend-role",
							"test-backend-role",
						},
					}
					transport.RegisterResponder(
						http.MethodGet,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						httpmock.NewJsonResponderOrPanic(200, responses.GetRoleMappingReponse{
							"test-role": roleMappingRequest,
						}).Times(2, failMessage),
					)
					transport.RegisterResponder(
						http.MethodPut,
						fmt.Sprintf(
							"https://%s.%s.svc.cluster.local:9200/_plugins/_security/api/rolesmapping/test-role",
							cluster.Spec.General.ServiceName,
							cluster.Namespace,
						),
						func(req *http.Request) (*http.Response, error) {
							mapping := &requests.RoleMapping{}
							if err := json.NewDecoder(req.Body).Decode(&mapping); err != nil {
								return httpmock.NewStringResponse(501, ""), nil
							}
							users = mapping.Users
							backendRoles = mapping.BackendRoles
							return httpmock.NewStringResponse(200, ""), nil
						},
					)
				})
				It("should remove the user and update the mapping", func() {
					Expect(reconciler.Delete()).To(Succeed())
					Expect(transport.GetTotalCallCount()).To(Equal(transport.NumResponders() + extraContextCalls + 1))
					Expect(backendRoles).To(ContainElement("someother-backend-role"))
					Expect(backendRoles).NotTo(ContainElement("test-backend-role"))
					Expect(users).To(BeEmpty())
				})
			})
		})
	})
})
