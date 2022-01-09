package main

import (
	"crypto/tls"
	"fmt"
	"github.com/opensearch-project/opensearch-go"
	"net/http"
	"opensearch-k8-operator/opensearch-gateway/services"
)

func main() {
	// Initialize the client with SSL/TLS enabled.
	config := opensearch.Config{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Addresses: []string{"https://localhost:9111"},
		Username:  "admin", // For testing only. Don't store credentials in code.
		Password:  "admin",
	}

	dataService, err := services.NewOsClusterClient(config)
	if err == nil {
		mainPage := dataService.MainPage
		fmt.Printf("main page:%v", mainPage)
	}

}
