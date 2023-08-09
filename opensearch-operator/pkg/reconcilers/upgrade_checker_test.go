package reconcilers

import (
	"encoding/xml"
	"fmt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
)

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

	body, err := io.ReadAll(response.Body)
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
