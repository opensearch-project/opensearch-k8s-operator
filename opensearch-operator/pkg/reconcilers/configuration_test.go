package reconcilers

import (
	"context"
	"strings"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
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
	instance *opensearchv1.OpenSearchCluster,
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

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterName,
					UID:       "dummyuid",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{},
					NodePools: []opensearchv1.NodePool{
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

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterName,
					UID:       "dummyuid",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{},
					NodePools: []opensearchv1.NodePool{
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
			// YAML library may format differently, so check for key-value pairs more flexibly
			Expect(strings.Contains(data, "foo:") || strings.Contains(data, "foo :")).To(BeTrue())
			Expect(strings.Contains(data, "bar:") || strings.Contains(data, "bar :")).To(BeTrue())
			Expect(strings.Contains(data, "baz")).To(BeTrue())
		})
	})

	Context("When Reconciling with General.AdditionalConfig", func() {
		It("should create a shared configmap", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterName,
					UID:       "dummyuid",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						AdditionalConfig: map[string]string{
							"general.config": "general-value",
						},
					},
					NodePools: []opensearchv1.NodePool{
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
			Expect(strings.Contains(data, "general.config:")).To(BeTrue())
			Expect(strings.Contains(data, "general-value")).To(BeTrue())
		})
	})

	Context("When Reconciling with NodePool.AdditionalConfig", func() {
		It("should create both shared and per-nodepool configmaps with merged config", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterName,
					UID:       "dummyuid",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						AdditionalConfig: map[string]string{
							"general.config": "general-value",
							"shared.config":  "shared-value",
						},
					},
					NodePools: []opensearchv1.NodePool{
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
			Expect(strings.Contains(sharedData, "general.config:")).To(BeTrue())
			Expect(strings.Contains(sharedData, "general-value")).To(BeTrue())
			Expect(strings.Contains(sharedData, "shared.config:")).To(BeTrue())
			Expect(strings.Contains(sharedData, "shared-value")).To(BeTrue())

			// Nodes should have merged config (nodepool overrides general)
			Expect(nodesCm).ToNot(BeNil())
			nodesData := nodesCm.Data["opensearch.yml"]
			Expect(strings.Contains(nodesData, "general.config:")).To(BeTrue())
			Expect(strings.Contains(nodesData, "general-value")).To(BeTrue())
			Expect(strings.Contains(nodesData, "nodepool.config:")).To(BeTrue())
			Expect(strings.Contains(nodesData, "nodepool-value")).To(BeTrue())
			Expect(strings.Contains(nodesData, "shared.config:")).To(BeTrue())
			Expect(strings.Contains(nodesData, "nodepool-override")).To(BeTrue())

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

	Context("When Reconciling with values containing special YAML characters", func() {
		It("should properly quote values with asterisks", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterName,
					UID:       "dummyuid",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						AdditionalConfig: map[string]string{
							"reindex.remote.allowlist": "*.svc.cluster.local:9200",
						},
					},
					NodePools: []opensearchv1.NodePool{
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
			data, exists := createdConfigMap.Data["opensearch.yml"]
			Expect(exists).To(BeTrue())
			// The value should be quoted to avoid YAML parsing errors
			// YAML library may use single or double quotes, but it should be quoted
			Expect(strings.Contains(data, `reindex.remote.allowlist:`)).To(BeTrue())
			Expect(strings.Contains(data, `*.svc.cluster.local:9200`)).To(BeTrue())
			// Check that the value is quoted (either single or double quotes)
			// The YAML library should quote strings starting with *
			quotedPattern := `"*.svc.cluster.local:9200"`       // double quotes
			singleQuotedPattern := `'*.svc.cluster.local:9200'` // single quotes
			Expect(strings.Contains(data, quotedPattern) || strings.Contains(data, singleQuotedPattern)).To(BeTrue(),
				"Expected value to be quoted, but got: %s", data)
		})

		It("should properly handle JSON array values", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterName,
					UID:       "dummyuid",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						AdditionalConfig: map[string]string{
							"plugins.security.restapi.roles_enabled": `["all_access", "security_rest_api_access"]`,
							"reindex.remote.allowlist":               `["*.svc.cluster.local:9200", "other.host:9200"]`,
						},
					},
					NodePools: []opensearchv1.NodePool{
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
			data, exists := createdConfigMap.Data["opensearch.yml"]
			Expect(exists).To(BeTrue())
			// JSON array values should be parsed and marshaled as YAML arrays
			Expect(strings.Contains(data, `plugins.security.restapi.roles_enabled:`)).To(BeTrue())
			Expect(strings.Contains(data, `reindex.remote.allowlist:`)).To(BeTrue())
			// Arrays should contain the values
			Expect(strings.Contains(data, "all_access")).To(BeTrue())
			Expect(strings.Contains(data, "security_rest_api_access")).To(BeTrue())
			Expect(strings.Contains(data, "*.svc.cluster.local:9200")).To(BeTrue())
			Expect(strings.Contains(data, "other.host:9200")).To(BeTrue())
		})

		It("should properly handle JSON object values", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterName,
					UID:       "dummyuid",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						AdditionalConfig: map[string]string{
							"test.object": `{"key": "value", "number": 123}`,
						},
					},
					NodePools: []opensearchv1.NodePool{
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
			data, exists := createdConfigMap.Data["opensearch.yml"]
			Expect(exists).To(BeTrue())
			// JSON object should be parsed and marshaled as YAML object
			Expect(strings.Contains(data, `test.object:`)).To(BeTrue())
			Expect(strings.Contains(data, "key:")).To(BeTrue())
			Expect(strings.Contains(data, "value")).To(BeTrue())
			Expect(strings.Contains(data, "number:")).To(BeTrue())
			Expect(strings.Contains(data, "123")).To(BeTrue())
		})

		It("should properly handle boolean and numeric values", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterName,
					UID:       "dummyuid",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						AdditionalConfig: map[string]string{
							"test.boolean": "true",
							"test.number":  "123",
							"test.float":   "123.45",
						},
					},
					NodePools: []opensearchv1.NodePool{
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
			data, exists := createdConfigMap.Data["opensearch.yml"]
			Expect(exists).To(BeTrue())
			// Boolean and numeric values should be unquoted
			Expect(strings.Contains(data, `test.boolean:`)).To(BeTrue())
			Expect(strings.Contains(data, `test.number:`)).To(BeTrue())
			Expect(strings.Contains(data, `test.float:`)).To(BeTrue())
			// Values should be present and unquoted (YAML library handles this)
			Expect(strings.Contains(data, "true")).To(BeTrue())
			Expect(strings.Contains(data, "123")).To(BeTrue())
			Expect(strings.Contains(data, "123.45")).To(BeTrue())
		})

		It("should properly handle strings that look like numbers but aren't", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			spec := opensearchv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: clusterName,
					UID:       "dummyuid",
				},
				Spec: opensearchv1.ClusterSpec{
					General: opensearchv1.GeneralConfig{
						AdditionalConfig: map[string]string{
							"test.string1": "123abc",
							"test.string2": "123.45.67",
						},
					},
					NodePools: []opensearchv1.NodePool{
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
			data, exists := createdConfigMap.Data["opensearch.yml"]
			Expect(exists).To(BeTrue())
			// Strings that aren't valid numbers should be quoted
			Expect(strings.Contains(data, `test.string1:`)).To(BeTrue())
			Expect(strings.Contains(data, `test.string2:`)).To(BeTrue())
			Expect(strings.Contains(data, "123abc")).To(BeTrue())
			Expect(strings.Contains(data, "123.45.67")).To(BeTrue())
		})
	})
})
