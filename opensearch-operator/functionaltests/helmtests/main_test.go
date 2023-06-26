package helmtests

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/clientcmd"
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
	k8sClient, err = client.New(config, client.Options{})
	if err != nil {
		panic(err.Error())
	}
	RegisterFailHandler(Fail)

	RunSpecs(t, "HelmTests")
}
