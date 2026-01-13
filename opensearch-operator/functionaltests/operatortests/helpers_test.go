package operatortests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	opsterv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/util/intstr"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clog "sigs.k8s.io/controller-runtime/pkg/log"
)

// getManifestPath returns the full path to a manifest file
// If name is a simple name (no path separators), prepends "resources/" prefix
// Adds .yaml extension if not present
func getManifestPath(name string) string {
	var filePath string

	// If name contains a path separator, use it as-is
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		filePath = name
	} else {
		// Simple name: add resources/ prefix
		filePath = "resources/" + name
	}

	// Add .yaml extension if not present
	if !strings.HasSuffix(filePath, ".yaml") && !strings.HasSuffix(filePath, ".yml") {
		filePath = filePath + ".yaml"
	}

	return filePath
}

func CreateKubernetesObjects(name string) error {
	filePath := getManifestPath(name)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read manifest file %s: %w", filePath, err)
	}
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(data), 100)
	for {
		var rawObj runtime.RawExtension
		if err = decoder.Decode(&rawObj); err != nil {
			break
		}

		obj, _, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			log.Fatal(err)
		}
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			log.Fatal(err)
		}

		unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}

		if err = k8sClient.Create(context.Background(), unstructuredObj); err != nil {
			// If resource already exists, skip it (useful when SKIP_CLEANUP is set)
			if apierrors.IsAlreadyExists(err) {
				continue
			}
			log.Fatal(err)
		}
	}
	if err != io.EOF {
		log.Fatal("eof ", err)
	}
	return nil
}

// ShouldSkipCleanup checks if cleanup should be skipped based on the SKIP_CLEANUP environment variable.
// Returns true if SKIP_CLEANUP is set to "true", "1", or "yes" (case-insensitive).
// Defaults to false (cleanup enabled) if the variable is not set or has any other value.
func ShouldSkipCleanup() bool {
	skipCleanup := strings.ToLower(strings.TrimSpace(os.Getenv("SKIP_CLEANUP")))
	return skipCleanup == "true" || skipCleanup == "1" || skipCleanup == "yes"
}

func Cleanup(name string) {
	filePath := getManifestPath(name)

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to read manifest file %s: %w", filePath, err))
	}
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(data), 100)
	for {
		var rawObj runtime.RawExtension
		if err = decoder.Decode(&rawObj); err != nil {
			break
		}

		obj, _, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			log.Fatal(err)
		}
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			log.Fatal(err)
		}

		unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}
		// Ignore errors as we don't care at this point
		_ = k8sClient.Delete(context.Background(), unstructuredObj)
	}
	if err != io.EOF {
		log.Fatal("eof ", err)
	}
}

func Get(obj client.Object, key client.ObjectKey, timeout time.Duration) {
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), key, obj)
		return err == nil
	}, timeout, time.Second*1).Should(BeTrue())
}

func ExposePodViaNodePort(selector map[string]string, namespace string, nodePort, targetPort int32) error {
	serviceName := fmt.Sprintf("nodeport-%d", nodePort)
	service := corev1.Service{}

	// Check if service already exists
	err := k8sClient.Get(context.Background(), client.ObjectKey{Name: serviceName, Namespace: namespace}, &service)
	if err == nil {
		// Service already exists, return success
		return nil
	}
	if !apierrors.IsNotFound(err) {
		// Some other error occurred
		return err
	}

	// Service doesn't exist, create it
	service = corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "nodeport",
					NodePort:   nodePort,
					Port:       targetPort,
					TargetPort: intstr.FromInt(int(targetPort)),
				},
			},
			Selector: selector,
			Type:     corev1.ServiceTypeNodePort,
		},
	}
	return k8sClient.Create(context.Background(), &service)
}

func CleanUpNodePort(namespace string, nodePort int32) error {
	service := corev1.Service{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: fmt.Sprintf("nodeport-%d", nodePort)}, &service); err != nil {
		return err
	}
	return k8sClient.Delete(context.Background(), &service)
}

func SetNestedKey(obj map[string]interface{}, value string, keys ...string) error {
	var m = obj
	for idx, key := range keys {
		if idx == len(keys)-1 {
			m[key] = value
			return nil
		} else {
			m = m[key].(map[string]interface{})
		}
	}
	return nil
}

// WaitForClusterReady waits for the cluster to be ready
func WaitForClusterReady(k8sClient client.Client, clusterName, namespace string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for cluster to be ready")
		default:
			cluster := &opsterv1.OpenSearchCluster{}
			if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, cluster); err != nil {
				time.Sleep(2 * time.Second)
				continue
			}

			// Check if cluster is ready (you might need to adjust this based on your cluster status)
			// For now, we'll just check if we can connect
			manager, err := NewTestDataManager(k8sClient, clusterName, namespace)
			if err == nil {
				health, err := manager.osClient.GetHealth()
				if err == nil && (health.Status == "green" || health.Status == "yellow") {
					return nil
				}
			}

			time.Sleep(2 * time.Second)
		}
	}
}

// getContextWithLogger returns a context with a logger
func getContextWithLogger() context.Context {
	logger := logr.Discard()
	ctx := clog.IntoContext(context.Background(), logger)
	// Also add to klog context for client-go compatibility
	ctx = klog.NewContext(ctx, logger)
	return ctx
}

var (
	nodePortOnce sync.Once
	nodePortURL  string
	nodePortErr  error
)

// getAccessibleClusterURL returns a cluster URL that can be accessed from outside the k3d cluster.
// For k3d clusters, we expose the OpenSearch service via a NodePort and access it through localhost.
func getAccessibleClusterURL(k8sClient client.Client, cluster *opsterv1.OpenSearchCluster) (string, error) {
	httpPort := cluster.Spec.General.HttpPort
	if httpPort == 0 {
		httpPort = 9200
	}
	protocol := "https"
	if !helpers.IsHttpTlsEnabled(cluster) {
		protocol = "http"
	}

	const nodePort int32 = 30000 // must match k3d port mapping (30000-30005)

	// Expose the OpenSearch HTTP service via NodePort exactly once
	nodePortOnce.Do(func() {
		selector := map[string]string{
			helpers.ClusterLabel: cluster.Name,
		}

		// Ignore errors in subsequent calls; NodePort is created once
		nodePortErr = ExposePodViaNodePort(selector, cluster.Namespace, nodePort, httpPort)
		if nodePortErr == nil {
			nodePortURL = fmt.Sprintf("%s://127.0.0.1:%d", protocol, nodePort)
		}
	})

	if nodePortErr != nil {
		return "", nodePortErr
	}

	// Give the service a moment to be ready before trying to connect
	time.Sleep(2 * time.Second)

	return nodePortURL, nil
}

// getDefaultTestData returns default test data that can be used across scenarios
func getDefaultTestData() []TestIndex {
	return []TestIndex{
		{
			Name: "test-products",
			Documents: []map[string]interface{}{
				{"id": "1", "name": "Laptop", "price": 999.99, "category": "Electronics", "stock": 50},
				{"id": "2", "name": "Mouse", "price": 29.99, "category": "Electronics", "stock": 200},
				{"id": "3", "name": "Keyboard", "price": 79.99, "category": "Electronics", "stock": 150},
			},
		},
		{
			Name: "test-orders",
			Documents: []map[string]interface{}{
				{"id": "1", "orderId": "ORD-001", "customerId": "CUST-001", "total": 1029.98, "status": "completed"},
				{"id": "2", "orderId": "ORD-002", "customerId": "CUST-002", "total": 79.99, "status": "pending"},
				{"id": "3", "orderId": "ORD-003", "customerId": "CUST-001", "total": 29.99, "status": "shipped"},
			},
		},
		{
			Name: "test-users",
			Documents: []map[string]interface{}{
				{"id": "1", "username": "alice", "email": "alice@example.com", "role": "admin", "active": true},
				{"id": "2", "username": "bob", "email": "bob@example.com", "role": "user", "active": true},
				{"id": "3", "username": "charlie", "email": "charlie@example.com", "role": "user", "active": false},
			},
		},
	}
}

// setupDataIntegrityTest initializes common test setup for data integrity tests
func setupDataIntegrityTest(clusterName, namespace string) (*TestDataManager, *ClusterOperations) {
	By("Waiting for master node pool to be ready (3 replicas)")
	Eventually(func() bool {
		sts := appsv1.StatefulSet{}
		err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-masters", Namespace: namespace}, &sts)
		if err != nil {
			return false
		}
		ready := sts.Status.ReadyReplicas
		if ready < 3 {
			GinkgoWriter.Printf("    Master nodes: %d/3 ready\n", ready)
		}
		return ready == 3
	}, time.Minute*15, time.Second*5).Should(BeTrue())
	GinkgoWriter.Printf("  + Master node pool ready: 3/3 replicas\n")

	By("Waiting for data node pool to be ready (3 replicas)")
	Eventually(func() bool {
		sts := appsv1.StatefulSet{}
		err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-data", Namespace: namespace}, &sts)
		if err != nil {
			return false
		}
		ready := sts.Status.ReadyReplicas
		if ready < 3 {
			GinkgoWriter.Printf("    Data nodes: %d/3 ready\n", ready)
		}
		return ready == 3
	}, time.Minute*15, time.Second*5).Should(BeTrue())
	GinkgoWriter.Printf("  + Data node pool ready: 3/3 replicas\n")

	By("Initializing test data manager")
	dataManager, err := NewTestDataManager(k8sClient, clusterName, namespace)
	Expect(err).NotTo(HaveOccurred())
	GinkgoWriter.Printf("  + Test data manager initialized\n")

	By("Initializing cluster operations helper")
	operations := NewClusterOperations(k8sClient, namespace)
	GinkgoWriter.Printf("  + Cluster operations helper initialized\n")

	return dataManager, operations
}
