package helpers

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	opsterv1 "opensearch.opster.io/api/v1"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"time"
)

var (
	K8sClient client.Client // You'll be using this client in your tests.
	testEnv   *envtest.Environment
)

func BeforeSuiteLogic() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	//logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	ctx := context.Background()
	By("bootstrappifng test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	fmt.Println(err)
	Expect(cfg).NotTo(BeNil())
	if err != nil {
		fmt.Println(err)
	}

	err = scheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = opsterv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	K8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(K8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

	K8sClient = k8sManager.GetClient()
	Expect(K8sClient).ToNot(BeNil())

}

func AfterSuiteLogic() {
	By("tearing down the test environment")
	gexec.KillAndWait(5 * time.Second)
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
}

func IsCreated(ctx context.Context, k8sClient client.Client, obj client.Object) bool {
	if err := k8sClient.Get(ctx, client.ObjectKey{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}, obj); err != nil {
		return false
	}
	return true
}

func IsNsDeleted(k8sClient client.Client, namespace corev1.Namespace) bool {
	ns := corev1.Namespace{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: namespace.Name}, &ns); err == nil {
		return ns.Status.Phase == "Terminating"
	}
	return true
}

func IsNsCreated(k8sClient client.Client, namespace corev1.Namespace) bool {
	ns := corev1.Namespace{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: namespace.Name}, &ns); err == nil {
		return true
	} else {
		return false
	}
}

func IsClusterCreated(k8sClient client.Client, cluster opsterv1.OpenSearchCluster) bool {
	ns := corev1.Namespace{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, &ns); err == nil {
		return true
	} else {
		return false
	}
}

func ComposeOpensearchCrd(ClusterName string, ClusterNameSpaces string) opsterv1.OpenSearchCluster {

	OpensearchCluster := &opsterv1.OpenSearchCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OpenSearchCluster",
			APIVersion: "opensearch.opster.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterNameSpaces,
		},
		Spec: opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
				ClusterName: ClusterName,
				HttpPort:    9200,
				Vendor:      "opensearch",
				Version:     "latest",
				ServiceName: "es-svc",
			},
			ConfMgmt: opsterv1.ConfMgmt{
				AutoScaler: false,
				Monitoring: false,
				VerUpdate:  false,
			},
			Dashboards: opsterv1.DashboardsConfig{Enable: true},
			NodePools: []opsterv1.NodePool{{
				Component:    "master",
				Replicas:     3,
				DiskSize:     32,
				NodeSelector: "",
				Cpu:          4,
				Memory:       16,
				Roles: []string{
					"master",
					"data",
				}}, {
				Component:    "nodes",
				Replicas:     3,
				DiskSize:     32,
				NodeSelector: "",
				Cpu:          4,
				Memory:       16,
				Roles: []string{
					"data",
				}}, {
				Component:    "client",
				Replicas:     3,
				DiskSize:     32,
				NodeSelector: "",
				Cpu:          4,
				Memory:       16,
				Roles: []string{
					"data",
					"ingest",
				},
			}},
		},
		Status: opsterv1.ClusterStatus{
			ComponentsStatus: []opsterv1.ComponentStatus{{
				Component:   "",
				Status:      "",
				Description: "",
			},
			},
		},
	}
	return *OpensearchCluster

}

func ComposeNs(name string) corev1.Namespace {

	return corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func DataNodeSize(cluster opsterv1.OpenSearchCluster) int {
	dataNodesSize := 0
	for i := 0; i < len(cluster.Spec.NodePools); i++ {
		foundData := false
		for j := 0; j < len(cluster.Spec.NodePools[i].Roles) && !foundData; j++ {
			if cluster.Spec.NodePools[i].Roles[j] == "data" {
				dataNodesSize++
				foundData = true
			}
		}
	}
	return dataNodesSize

}
