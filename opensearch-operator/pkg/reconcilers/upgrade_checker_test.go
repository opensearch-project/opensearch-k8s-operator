package reconcilers

import (
	"encoding/xml"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"testing"
)

//func newUpgradeCheckerReconciler(spec *opsterv1.OpenSearchCluster) (ReconcilerContext, *UpgradeCheckerReconciler) {
//	reconcilerContext := NewReconcilerContext(spec.Spec.NodePools)
//	underTest := NewUpgradeCheckerReconciler(
//		k8sClient,
//		context.Background(),
//		&helpers.MockEventRecorder{},
//		&reconcilerContext,
//		spec,
//	)
//	underTest.pki = helpers.NewMockPKI()
//	return reconcilerContext, underTest
//}

type RequestBody struct {
	UID                string            `json:"uid"`
	OperatorVersion    string            `json:"operatorVersion"`
	ClusterCount       int               `json:"clusterCount"`
	OSClustersVersions map[string]string `json:"osClustersVersions"`
}

//var _ = Describe("UpgradeChecker Controller", func() {
//
//	// Define utility constants for object names and testing timeouts/durations and intervals.
//	const (
//		timeout  = time.Second * 30
//		interval = time.Second * 1
//		url      = "http://upgrade-chcker-dev.opster.co/operator-usage"
//	)
//
//	Context("When Reconciling Cluster with UpgradeChecker enable ", func() {
//		It("it should send details to UpgradeChecker server and retun true/false", func() {
//			clusterName := "upgradecheck"
//
//			spec := opsterv1.OpenSearchCluster{
//				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
//				Spec: opsterv1.ClusterSpec{
//					General: opsterv1.GeneralConfig{},
//				}}
//			Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())
//			_, underTest := newUpgradeCheckerReconciler(&spec)
//			_, err := underTest.Reconcile()
//			Expect(err).ToNot(HaveOccurred())
//
//			Eventually(func() bool {
//				operatorDeploy := appsv1.Deployment{}
//				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: "opensearch-operator-controller-manager", Namespace: clusterName}, &operatorDeploy)
//				if err != nil {
//					return false
//				}
//
//				// func that checks if the UpgradeChecker is enabled
//				// if it enabled checking the version of the install operator,and preform demo Json call with the version and i check the response
//				// if version == latest so i should return true, else it will return false
//
//				for _, container := range operatorDeploy.Spec.Template.Spec.Containers {
//					for _, env := range container.Env {
//						if env.Name == "UpgradeChecker" {
//							if env.Value == "true" {
//								image := container.Image
//								islatest, err, version := GetlatestVersion(image)
//								if err != nil {
//									return false
//								}
//								data, headers := BuildPayload(version)
//								response, err := performRequest(url, "POST", data, headers)
//								if err != nil {
//									fmt.Println("Error performing request:", err)
//									return false
//								}
//								// if version latest check the response is true
//								if islatest {
//									if response == "true" {
//										return true
//									} else {
//										return false
//									}
//									// if version IS NOT latest check the response is true
//								} else if response == "true" {
//									return true
//								}
//							} else {
//								return false
//							}
//						}
//					}
//				}
//				return true
//			}, timeout, interval).Should(BeTrue())
//
//		})
//	})
//})
//
//func performRequest(url string, method string, data []byte, headers map[string]string) (string, error) {
//	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
//	if err != nil {
//		return "", fmt.Errorf("error creating request: %v", err)
//	}
//
//	// Set headers if provided
//	for key, value := range headers {
//		req.Header.Set(key, value)
//	}
//
//	client := &http.Client{}
//	resp, err := client.Do(req)
//	if err != nil {
//		return "", fmt.Errorf("error sending request: %v", err)
//	}
//	defer resp.Body.Close()
//
//	body, err := io.ReadAll(resp.Body)
//	if err != nil {
//		return "", fmt.Errorf("error reading response: %v", err)
//	}
//
//	return string(body), nil
//}
//
//type RSS struct {
//	Channel Channel `xml:"channel"`
//}
//
//type Channel struct {
//	Items []Item `xml:"item"`
//}
//
//func GetlatestVersion(installedVersion string) (bool, error, string) {
//	// Fetch the RSS feed
//	resp, err := http.Get("https://artifacthub.io/api/v1/packages/helm/opensearch-operator/opensearch-operator/feed/rss")
//	if err != nil {
//		fmt.Println("Failed to fetch RSS feed:", err)
//		return false, err, ""
//	}
//	defer resp.Body.Close()
//
//	// Read the response body
//	body, err := io.ReadAll(resp.Body)
//	if err != nil {
//		fmt.Println("Failed to read response body:", err)
//		return false, err, ""
//	}
//
//	// Parse the XML
//	var rss RSS
//	err = xml.Unmarshal(body, &rss)
//	if err != nil {
//		fmt.Println("Failed to parse XML:", err)
//		return false, err, ""
//	}
//	var opVersion string
//	// Extract the OpVersion from the first item
//	if len(rss.Channel.Items) > 0 {
//		opVersion = rss.Channel.Items[0].Title
//		fmt.Println("OpVersion:", opVersion)
//	} else {
//		fmt.Println("No items found in the RSS feed")
//		return false, err, ""
//
//	}
//	if opVersion == installedVersion {
//		return true, nil, installedVersion
//	}
//	return false, err, installedVersion
//
//}
//
//func BuildPayload(version string) (data []byte, headers map[string]string) {
//	requestBody := RequestBody{
//		UID:             "myUid",
//		OperatorVersion: version,
//		ClusterCount:    2,
//		OSClustersVersions: map[string]string{
//			"version1": "os1",
//			"version2": "os2",
//		},
//	}
//	data, err := json.Marshal(requestBody)
//	if err != nil {
//		fmt.Println("Error creating request body:", err)
//		return nil, nil
//	}
//	headers = map[string]string{
//		"Content-Type": "application/json",
//	}
//	return data, headers
//}

type ServerClient interface {
	SendJSON(jsonData []byte) (bool, error)
}

type MockServerClient struct {
	mock.Mock
}

// is the mock implementation for SendJSON .
func (m *MockServerClient) SendJSON(jsonData []byte) (bool, error) {
	args := m.Called(jsonData)
	return args.Bool(0), args.Error(1)
}

// SendJSONToServer sends the JSON data to the server using the provided client.
func SendJSONToServer(client ServerClient, operatorVersion string) bool {
	// Fetch the latest version from the URL
	versionURL := "https://artifacthub.io/api/v1/packages/helm/opensearch-operator/opensearch-operator/feed/rss"
	latestVersion, err := FetchLatestVersion(versionURL)
	if err != nil {
		log.Printf("Error fetching the latest version: %v", err)
		return false
	}

	// Compare the operatorVersion with the latest version
	if latestVersion == operatorVersion {
		log.Println("The operator version is up to date.")
		return true
	} else if strings.Compare(latestVersion, operatorVersion) > 0 {
		log.Println("The operator version is below the latest version. - OperatorVersion :, Latestsversion : ", operatorVersion, latestVersion)
		return false
	}

	log.Println("Invalid operator version.")
	return false
}

func TestSendJSONToServer(t *testing.T) {
	mockClient := new(MockServerClient)

	mockClient.On("SendJSON", mock.Anything).Return(true, nil)

	versionURL := "https://artifacthub.io/api/v1/packages/helm/opensearch-operator/opensearch-operator/feed/rss"
	latestVersion, err := FetchLatestVersion(versionURL)
	if err != nil {
		log.Printf("Error fetching the latest version: %v", err)
	}

	// Test with latest version to see that the function is working and answering right
	result := SendJSONToServer(mockClient, latestVersion)
	assert.True(t, result)

	// Test with a specific version - should be fail
	result = SendJSONToServer(mockClient, "1.0.0")
	assert.False(t, result)

}

type RSSFeed struct {
	XMLName xml.Name `xml:"rss"`
	Items   []Item   `xml:"channel>item"`
}

// Item represents an item in the RSS feed.
type Item struct {
	Title string `xml:"title"`
	Link  string `xml:"link"`
}

// fetches the latest version from the given URL.
func FetchLatestVersion(url string) (string, error) {
	// Make a GET request to the URL
	response, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	var feed RSSFeed
	err = xml.Unmarshal(body, &feed)
	if err != nil {
		return "", err
	}
	if len(feed.Items) > 0 {
		latestVersion := feed.Items[0].Title
		return latestVersion, nil
	}

	return "", fmt.Errorf("no items found in the RSS feed")
}
