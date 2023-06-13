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

	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/phayes/freeport"
	opsterv1 "opensearch.opster.io/api/v1"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/cisco-open/operator-tools/pkg/prometheus"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
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

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var cancel context.CancelFunc

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reconciler Suite")
}

var _ = BeforeSuite(func() {

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ports, err := freeport.GetFreePorts(2)
	Expect(err).NotTo(HaveOccurred())

	serviceMonitorCRD := apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: prometheus.ServiceMonitorName + "." + prometheus.GroupVersion.Group,
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: prometheus.GroupVersion.Group,
			Scope: apiextensionsv1.NamespaceScoped,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   prometheus.ServiceMonitorName,
				Singular: prometheus.ServiceMonitorKindKey,
				Kind:     prometheus.ServiceMonitorsKind,
			},
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    prometheus.GroupVersion.Version,
					Storage: true,
					Served:  true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Properties:             map[string]apiextensionsv1.JSONSchemaProps{},
							XPreserveUnknownFields: pointer.Bool(true),
							Type:                   "object",
						},
					},
				},
			},
		},
	}

	var ctx context.Context
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDs:                  []*apiextensionsv1.CustomResourceDefinition{&serviceMonitorCRD},
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = scheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = opsterv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	//err = prometheus.AddToScheme(scheme.Scheme)
	//Expect(err).NotTo(HaveOccurred())

	err = monitoring.AddToScheme(scheme.Scheme)
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

})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func CreateNamespace(k8sClient client.Client, name string) error {
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	return k8sClient.Create(context.Background(), &ns)
}

func failMessage(mesg ...interface{}) {
	Fail(fmt.Sprintln(mesg...))
}
