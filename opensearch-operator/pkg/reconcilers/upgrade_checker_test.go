package reconcilers

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newUpgradeCheckerReconciler(spec *opsterv1.OpenSearchCluster) (ReconcilerContext, *UpgradeCheckerReconciler) {
	reconcilerContext := NewReconcilerContext(spec.Spec.NodePools)
	underTest := NewUpgradeCheckerReconciler(
		k8sClient,
		context.Background(),
		&helpers.MockEventRecorder{},
		&reconcilerContext,
		spec,
	)
	underTest.pki = helpers.NewMockPKI()
	return reconcilerContext, underTest
}

type RequestBody struct {
	UID                string            `json:"uid"`
	OperatorVersion    string            `json:"operatorVersion"`
	ClusterCount       int               `json:"clusterCount"`
	OSClustersVersions map[string]string `json:"osClustersVersions"`
}

var _ = Describe("UpgradeChecker Controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		timeout  = time.Second * 30
		interval = time.Second * 1
		url      = "http://upgrade-chcker-dev.opster.co/operator-usage"
	)

	Context("When Reconciling Cluster with UpgradeChecker enable ", func() {
		It("it should send details to UpgradeChecker server and retun true/false", func() {
			clusterName := "upgradecheck"

			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{},
				}}
			Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())
			_, underTest := newUpgradeCheckerReconciler(&spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() bool {
				operatorDeploy := appsv1.Deployment{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: "opensearch-operator-controller-manager", Namespace: clusterName}, &operatorDeploy)
				if err != nil {
					return false
				}

				// func that checks if the UpgradeChecker is enabled
				// if it enabled checking the version of the install operator,and preform demo Json call with the version and i check the response
				// if version == latest so i should return true, else it will return false

				for _, container := range operatorDeploy.Spec.Template.Spec.Containers {
					for _, env := range container.Env {
						if env.Name == "UpgradeChecker" {
							if env.Value == "true" {
								image := container.Image
								islatest, err, version := GetlatestVersion(image)
								if err != nil {
									return false
								}
								data, headers := BuildPayload(version)
								response, err := performRequest(url, "POST", data, headers)
								if err != nil {
									fmt.Println("Error performing request:", err)
									return false
								}
								// if version latest check the response is true
								if islatest {
									if response == "true" {
										return true
									} else {
										return false
									}
									// if version IS NOT latest check the response is true
								} else if response == "true" {
									return true
								}
							} else {
								return false
							}
						}
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())

		})
	})
})

func performRequest(url string, method string, data []byte, headers map[string]string) (string, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	// Set headers if provided
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	return string(body), nil
}

type RSS struct {
	Channel Channel `xml:"channel"`
}

type Channel struct {
	Items []Item `xml:"item"`
}

type Item struct {
	Title string `xml:"title"`
}

func GetlatestVersion(installedVersion string) (bool, error, string) {
	// Fetch the RSS feed
	resp, err := http.Get("https://artifacthub.io/api/v1/packages/helm/opensearch-operator/opensearch-operator/feed/rss")
	if err != nil {
		fmt.Println("Failed to fetch RSS feed:", err)
		return false, err, ""
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Failed to read response body:", err)
		return false, err, ""
	}

	// Parse the XML
	var rss RSS
	err = xml.Unmarshal(body, &rss)
	if err != nil {
		fmt.Println("Failed to parse XML:", err)
		return false, err, ""
	}
	var opVersion string
	// Extract the OpVersion from the first item
	if len(rss.Channel.Items) > 0 {
		opVersion = rss.Channel.Items[0].Title
		fmt.Println("OpVersion:", opVersion)
	} else {
		fmt.Println("No items found in the RSS feed")
		return false, err, ""

	}
	if opVersion == installedVersion {
		return true, nil, installedVersion
	}
	return false, err, installedVersion

}

func BuildPayload(version string) (data []byte, headers map[string]string) {
	requestBody := RequestBody{
		UID:             "myUid",
		OperatorVersion: version,
		ClusterCount:    2,
		OSClustersVersions: map[string]string{
			"version1": "os1",
			"version2": "os2",
		},
	}
	data, err := json.Marshal(requestBody)
	if err != nil {
		fmt.Println("Error creating request body:", err)
		return nil, nil
	}
	headers = map[string]string{
		"Content-Type": "application/json",
	}
	return data, headers
}
