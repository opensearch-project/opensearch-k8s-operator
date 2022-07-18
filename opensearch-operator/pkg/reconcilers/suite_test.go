/*

First, it will contain the necessary imports.
*/

/*
Copyright 2022 The Kubernetes authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// +kubebuilder:docs-gen:collapse=Apache License
package reconcilers

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/onsi/gomega/gexec"
	"github.com/phayes/freeport"
	opsterv1 "opensearch.opster.io/api/v1"

	ctrl "sigs.k8s.io/controller-runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

// +kubebuilder:docs-gen:collapse=Imports

/*
Now, let's go through the code generated.
*/

var (
	k8sClient client.Client // You'll be using this client in your tests.
	testEnv   *envtest.Environment
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Reconciler Suite",
		[]Reporter{printer.NewlineReporter{}})

}

var _ = BeforeSuite(func() {

	//OpensearchCluster := ComposeOpensearchCrd("cluster-test", "default")

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ports, err := freeport.GetFreePorts(2)
	Expect(err).NotTo(HaveOccurred())
	//logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	ctx := context.Background()
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
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

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme.Scheme,
		MetricsBindAddress:     fmt.Sprintf(":%d", ports[0]),
		HealthProbeBindAddress: fmt.Sprintf(":%d", ports[1]),
	})
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	gexec.KillAndWait(5 * time.Second)
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func CreateNamespace(k8sClient client.Client, name string) error {
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	return k8sClient.Create(context.Background(), &ns)
}

func failMessage(mesg ...interface{}) {
	Fail(fmt.Sprintln(mesg...))
}
