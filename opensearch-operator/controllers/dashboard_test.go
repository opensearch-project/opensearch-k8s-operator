package controllers

import (
	"context"
	"fmt"
	sts "k8s.io/api/apps/v1"
	"k8s.io/utils/pointer"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var _ = Describe("Dashboards Reconciler", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		clusterName = "cluster-test-dash"
		namespace   = clusterName
		timeout     = time.Second * 30
		interval    = time.Second * 1
	)
	var (
		OpensearchCluster = ComposeOpensearchCrd(clusterName, namespace)
		cm                = corev1.ConfigMap{}
		service           = corev1.Service{}
		deploy            = sts.Deployment{}
	)

	/// ------- Creation Check phase -------

	Context("When create OpenSearch CRD - dash", func() {
		It("Should create the namespace first", func() {
			Expect(CreateNamespace(k8sClient, &OpensearchCluster)).Should(Succeed())
			By("Create cluster ns ")
			Eventually(func() bool {
				return IsNsCreated(k8sClient, namespace)
			}, timeout, interval).Should(BeTrue())
		})

		It("should create the secret for volumes", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: OpensearchCluster.Namespace,
				},
				StringData: map[string]string{
					"test.yml": "foobar",
				},
			}
			Expect(k8sClient.Create(context.Background(), secret)).To(Succeed())
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				}, &corev1.Secret{})
			}, timeout, interval).Should(Succeed())
		})

		It("should create the configmap for volumes", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: OpensearchCluster.Namespace,
				},
				Data: map[string]string{
					"test.yml": "foobar",
				},
			}
			Expect(k8sClient.Create(context.Background(), cm)).To(Succeed())
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      cm.Name,
					Namespace: cm.Namespace,
				}, &corev1.ConfigMap{})
			}, timeout, interval).Should(Succeed())
		})

		It("should apply the cluster instance successfully", func() {
			Expect(k8sClient.Create(context.Background(), &OpensearchCluster)).Should(Succeed())
		})
	})

	/// ------- Tests logic Check phase -------

	Context("When createing a OpenSearchCluster kind Instance - and Dashboard is Enable", func() {
		It("should create all Opensearch-dashboard resources", func() {
			//fmt.Println(OpensearchCluster)
			fmt.Println("\n DAShBOARD - START")

			By("Opensearch Dashboard")
			Eventually(func() bool {
				fmt.Println("\n DAShBOARD - START - 2")
				//// -------- Dashboard tests ---------
				if OpensearchCluster.Spec.Dashboards.Enable {
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: clusterName + "-dashboards"}, &deploy); err != nil {
						return false
					}
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: clusterName + "-dashboards-config"}, &cm); err != nil {
						return false
					}
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: clusterName, Name: OpensearchCluster.Spec.General.ServiceName + "-dashboards"}, &service); err != nil {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
		It("should set correct owner references", func() {
			Expect(HasOwnerReference(&deploy, &OpensearchCluster)).To(BeTrue())
			Expect(HasOwnerReference(&cm, &OpensearchCluster)).To(BeTrue())
			Expect(HasOwnerReference(&service, &OpensearchCluster)).To(BeTrue())
		})
		It("should create and configure dashboard deployment correctly", func() {
			if OpensearchCluster.Spec.Dashboards.Enable {
				dashboardDeployName := fmt.Sprintf("%s-dashboards", OpensearchCluster.Name)
				deployment := &sts.Deployment{}
				Eventually(func() error {
					return k8sClient.Get(context.Background(), types.NamespacedName{
						Name:      dashboardDeployName,
						Namespace: OpensearchCluster.Namespace,
					}, deployment)
				}, timeout, interval).Should(Succeed())
				Expect(deployment.Spec.Replicas).To(Equal(pointer.Int32(3)))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().String()).To(Equal("500m"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String()).To(Equal("1Gi"))
				Expect(deployment.Spec.Template.Spec.Tolerations).To(ContainElement(corev1.Toleration{
					Effect:   "NoSchedule",
					Key:      "foo",
					Operator: "Equal",
					Value:    "bar",
				}))
				Expect(deployment.Spec.Template.Spec.NodeSelector).Should(Equal(map[string]string{
					"foo": "bar",
				}))
				Expect(*deployment.Spec.Template.Spec.Affinity).To(Equal(corev1.Affinity{}))
			}
		})
	})
})
