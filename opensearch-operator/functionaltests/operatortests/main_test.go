package operatortests

import (
	"testing"

	opsterv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clog "sigs.k8s.io/controller-runtime/pkg/log"
)

var k8sClient client.Client

func TestAPIs(t *testing.T) {
	// Set up controller-runtime logger to avoid warnings
	clog.SetLogger(logr.Discard())

	config, err := clientcmd.BuildConfigFromFlags("", "../kubeconfig")
	if err != nil {
		panic(err.Error())
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	utilruntime.Must(opsterv1.AddToScheme(scheme))
	k8sClient, err = client.New(config, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		panic(err.Error())
	}
	RegisterFailHandler(Fail)

	RunSpecs(t, "FunctionalTests")
}
