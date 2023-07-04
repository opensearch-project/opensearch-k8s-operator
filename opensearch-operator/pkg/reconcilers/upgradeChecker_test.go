package reconcilers

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

type ServerClient interface {
	SendJSON(jsonData []byte) (bool, error)
}
type MockServerClient struct {
	mock.Mock
}

func (m *MockServerClient) SendJSON(jsonData []byte) (bool, error) {
	args := m.Called(jsonData)
	return args.Bool(0), args.Error(1)
}
func TestSendJSONToServer(t *testing.T) {
	// Create an instance of the mock
	mockClient := new(MockServerClient)

	// Set up expectations for the mock method
	mockClient.On("SendJSON", mock.Anything).Return(true, nil)

	// Call the function you want to test, passing the mock as the server client
	result := SendJSONToServer(mockClient)

	// Assert the result or perform any necessary verifications
	assert.True(t, result)

	// Verify that the expectations were met
	mockClient.AssertExpectations(t)
}

func SendJSONToServer(client ServerClient) bool {
	// Build your JSON data
	jsonData := []byte(`{
		"properties": {
			"clusterCount": "",
			"operatorVersion": "0",
			"osClustersVersions": [],
			"uid": ""
		}
	}`)

	// Send the JSON data to the server using the client
	result, err := client.SendJSON(jsonData)
	if err != nil {
		// Handle the error if needed
		// For example, log the error or return false
		return false
	}

	return result
}
