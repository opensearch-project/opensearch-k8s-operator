package operatortests

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	opsterv1 "opensearch.opster.io/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	k8sClient client.Client
)

func TestAPIs(t *testing.T) {
	config, err := clientcmd.BuildConfigFromFlags("", "../kubeconfig")
	if err != nil {
		panic(err.Error())
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
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
