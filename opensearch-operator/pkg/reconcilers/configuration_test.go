package reconcilers

import (
	"context"
	"strings"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/mocks/github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

func newConfigurationReconciler(
	client *k8s.MockK8sClient,
	recorder record.EventRecorder,
	reconcilerContext *ReconcilerContext,
	instance *opsterv1.OpenSearchCluster,
) *ConfigurationReconciler {
	return &ConfigurationReconciler{
		client:            client,
		reconcilerContext: reconcilerContext,
		recorder:          recorder,
		instance:          instance,
	}
}

var _ = Describe("Configuration Controller", func() {
	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		clusterName = "configuration-test"
	)

	Context("When Reconciling the configuration controller with no configuration snippets", func() {
		It("should not create a configmap ", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterName,
					UID:       "dummyuid",
				},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{},
					NodePools: []opsterv1.NodePool{
						{
							Component: "test",
							Roles: []string{
								"master",
								"data",
							},
						},
					},
				},
			}

			reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, &spec, spec.Spec.NodePools)

			underTest := newConfigurationReconciler(
				mockClient,
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				&spec,
			)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("When Reconciling the configuration controller with some configuration snippets", func() {
		It("should create a configmap ", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterName,
					UID:       "dummyuid",
				},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{},
					NodePools: []opsterv1.NodePool{
						{
							Component: "test",
							Roles: []string{
								"master",
								"data",
							},
						},
					},
				},
			}

			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().Context().Return(context.Background())
			var createdConfigMap *corev1.ConfigMap
			mockClient.On("CreateConfigMap", mock.Anything).
				Return(func(cm *corev1.ConfigMap) (*ctrl.Result, error) {
					createdConfigMap = cm
					return &ctrl.Result{}, nil
				})

			reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, &spec, spec.Spec.NodePools)

			underTest := newConfigurationReconciler(
				mockClient,
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				&spec,
			)
			reconcilerContext.AddConfig("foo", "bar")
			reconcilerContext.AddConfig("bar", "something")
			reconcilerContext.AddConfig("bar", "baz")
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			Expect(createdConfigMap).ToNot(BeNil())

			data, exists := createdConfigMap.Data["opensearch.yml"]
			Expect(exists).To(BeTrue())
			Expect(strings.Contains(data, "foo: bar\n")).To(BeTrue())
			Expect(strings.Contains(data, "bar: baz\n")).To(BeTrue())
		})
	})

	Context("When Reconciling with General.AdditionalConfig", func() {
		It("should create a shared configmap", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterName,
					UID:       "dummyuid",
				},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{
						AdditionalConfig: map[string]string{
							"general.config": "general-value",
						},
					},
					NodePools: []opsterv1.NodePool{
						{
							Component: "test",
							Roles: []string{
								"master",
								"data",
							},
						},
					},
				},
			}

			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().Context().Return(context.Background())
			var createdConfigMap *corev1.ConfigMap
			mockClient.On("CreateConfigMap", mock.Anything).
				Return(func(cm *corev1.ConfigMap) (*ctrl.Result, error) {
					createdConfigMap = cm
					return &ctrl.Result{}, nil
				})

			reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, &spec, spec.Spec.NodePools)

			underTest := newConfigurationReconciler(
				mockClient,
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				&spec,
			)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			Expect(createdConfigMap).ToNot(BeNil())
			Expect(createdConfigMap.Name).To(Equal(clusterName + "-config"))
			data, exists := createdConfigMap.Data["opensearch.yml"]
			Expect(exists).To(BeTrue())
			Expect(strings.Contains(data, "general.config: general-value\n")).To(BeTrue())
		})
	})

	Context("When Reconciling with NodePool.AdditionalConfig", func() {
		It("should create both shared and per-nodepool configmaps with merged config", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterName,
					UID:       "dummyuid",
				},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{
						AdditionalConfig: map[string]string{
							"general.config": "general-value",
							"shared.config":  "shared-value",
						},
					},
					NodePools: []opsterv1.NodePool{
						{
							Component: "masters",
							Roles: []string{
								"master",
								"data",
							},
						},
						{
							Component: "nodes",
							Roles: []string{
								"data",
							},
							AdditionalConfig: map[string]string{
								"nodepool.config": "nodepool-value",
								"shared.config":   "nodepool-override",
							},
						},
					},
				},
			}

			mockClient.EXPECT().Scheme().Return(scheme.Scheme)
			mockClient.EXPECT().Context().Return(context.Background())
			var createdConfigMaps []*corev1.ConfigMap
			mockClient.On("CreateConfigMap", mock.Anything).
				Return(func(cm *corev1.ConfigMap) (*ctrl.Result, error) {
					createdConfigMaps = append(createdConfigMaps, cm)
					return &ctrl.Result{}, nil
				})

			reconcilerContext := NewReconcilerContext(&helpers.MockEventRecorder{}, &spec, spec.Spec.NodePools)

			underTest := newConfigurationReconciler(
				mockClient,
				&helpers.MockEventRecorder{},
				&reconcilerContext,
				&spec,
			)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			// Should create shared configmap + per-nodepool configmap for nodes (2 total)
			// Masters nodepool doesn't have AdditionalConfig, so no per-nodepool configmap for it
			Expect(len(createdConfigMaps)).To(Equal(2))

			// Find shared and nodes configmaps
			var sharedCm *corev1.ConfigMap
			var nodesCm *corev1.ConfigMap
			for _, cm := range createdConfigMaps {
				switch cm.Name {
				case clusterName + "-config":
					sharedCm = cm
				case clusterName + "-nodes-config":
					nodesCm = cm
				}
			}

			// Shared configmap should have general config (for bootstrap and security update jobs)
			Expect(sharedCm).ToNot(BeNil())
			sharedData := sharedCm.Data["opensearch.yml"]
			Expect(strings.Contains(sharedData, "general.config: general-value\n")).To(BeTrue())
			Expect(strings.Contains(sharedData, "shared.config: shared-value\n")).To(BeTrue())

			// Nodes should have merged config (nodepool overrides general)
			Expect(nodesCm).ToNot(BeNil())
			nodesData := nodesCm.Data["opensearch.yml"]
			Expect(strings.Contains(nodesData, "general.config: general-value\n")).To(BeTrue())
			Expect(strings.Contains(nodesData, "nodepool.config: nodepool-value\n")).To(BeTrue())
			Expect(strings.Contains(nodesData, "shared.config: nodepool-override\n")).To(BeTrue())

			// Verify shared configmap volume is added to reconcilerContext
			Expect(reconcilerContext.Volumes).ToNot(BeEmpty())
			var hasConfigVolume bool
			for _, vol := range reconcilerContext.Volumes {
				if vol.Name == "config" && vol.ConfigMap != nil && vol.ConfigMap.Name == clusterName+"-config" {
					hasConfigVolume = true
					break
				}
			}
			Expect(hasConfigVolume).To(BeTrue())
		})
	})
})
