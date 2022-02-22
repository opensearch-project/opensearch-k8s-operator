package helpers

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	sts "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
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
	K8sClient  client.Client // You'll be using this client in your tests.
	testEnv    *envtest.Environment
	RestConfig *rest.Config
)

func BeforeSuiteLogic() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	//logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	ctx := ctrl.SetupSignalHandler()
	By("bootstrappifng test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}
	var err error = nil
	RestConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	fmt.Println(err)
	Expect(RestConfig).NotTo(BeNil())
	if err != nil {
		fmt.Println(err)
	}

	err = scheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = opsterv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	K8sClient, err = client.New(RestConfig, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(K8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(RestConfig, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

	//K8sClient = k8sManager.GetClient()
	//Expect(K8sClient).ToNot(BeNil())

}

func AfterSuiteLogic() {
	By("tearing down the test environment")
	gexec.KillAndWait(5 * time.Second)
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
}

func IsNsDeleted(k8sClient client.Client, namespace corev1.Namespace) bool {
	ns := corev1.Namespace{}
	if err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: namespace.Name, Namespace: namespace.Namespace}, &ns); err == nil {
		return ns.Status.Phase == "Terminating"
	}
	return true
}

func IsNsCreated(k8sClient client.Client, ctx context.Context, namespace corev1.Namespace) bool {
	ns := corev1.Namespace{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: namespace.Name}, &ns); err == nil {
		return true
	} else {
		return false
	}
}

func IsCrdCreated(k8sClient client.Client, cluster opsterv1.OpenSearchCluster) bool {
	ns := opsterv1.OpenSearchCluster{}
	if err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, &ns); err == nil {
		return true
	} else {
		return false
	}
}

func IsCmCreated(k8sClient client.Client, name string, namespace string) bool {
	res := corev1.ConfigMap{}
	if err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: namespace}, &res); err == nil {
		return true
	} else {
		return false
	}
}

func IsServiceCreated(k8sClient client.Client, name string, namespace string) bool {
	headlessService := v1.Service{}
	if err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: namespace}, &headlessService); err == nil {
		return true
	} else {
		return false
	}
}

func IsStsCreated(k8sClient client.Client, name string, namespace string) bool {
	_, err := GetSts(k8sClient, name, namespace)
	if err != nil {
		return false
	} else {
		return true
	}
}

func GetSts(k8sClient client.Client, name string, namespace string) (sts.StatefulSet, error) {
	sts := sts.StatefulSet{}
	if err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: namespace}, &sts); err == nil {
		return sts, nil
	} else {
		return sts, err
	}
}

func GetPod(k8sClient client.Client, name string, namespace string) (corev1.Pod, error) {
	pod := corev1.Pod{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: name, Namespace: namespace}, &pod); err == nil {
		return pod, nil
	} else {
		return pod, err
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
				}}},
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
func Retry(attempts int, sleep time.Duration, f func() bool) (err error) {
	for i := 0; i < attempts; i++ {
		result := f()
		if result {
			return nil
		}
		fmt.Println("retrying after error:", err)
		time.Sleep(sleep)
	}
	return fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}
