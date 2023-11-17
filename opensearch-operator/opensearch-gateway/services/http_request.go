package services

import (
	"context"
	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"io"
	"net/http"
	"strings"
)

// doHTTPGet performs a HTTP GET request
func doHTTPGet(ctx context.Context, client *opensearch.Client, path strings.Builder) (*opensearchapi.Response, error) {
	req, err := http.NewRequest(http.MethodGet, path.String(), nil)
	if err != nil {
		return nil, err
	}

	if ctx != nil {
		req = req.WithContext(ctx)
	}

	res, err := client.Perform(req)
	if err != nil {
		return nil, err
	}

	return &opensearchapi.Response{StatusCode: res.StatusCode, Body: res.Body, Header: res.Header}, nil
}

// doHTTPHead performs a HTTP HEAD request
func doHTTPHead(ctx context.Context, client *opensearch.Client, path strings.Builder) (*opensearchapi.Response, error) {
	req, err := http.NewRequest(http.MethodHead, path.String(), nil)
	if err != nil {
		return nil, err
	}

	if ctx != nil {
		req = req.WithContext(ctx)
	}

	res, err := client.Perform(req)
	if err != nil {
		return nil, err
	}

	return &opensearchapi.Response{StatusCode: res.StatusCode, Body: res.Body, Header: res.Header}, nil
}

// doHTTPPut performs a HTTP PUT request
func doHTTPPut(ctx context.Context, client *opensearch.Client, path strings.Builder, body io.Reader) (*opensearchapi.Response, error) {
	req, err := http.NewRequest(http.MethodPut, path.String(), body)
	if err != nil {
		return nil, err
	}

	if ctx != nil {
		req = req.WithContext(ctx)
	}
	req.Header.Add(headerContentType, jsonContentHeader)

	res, err := client.Perform(req)
	if err != nil {
		return nil, err
	}

	return &opensearchapi.Response{StatusCode: res.StatusCode, Body: res.Body, Header: res.Header}, nil
}

// doHTTPDelete performs a HTTP DELETE request
func doHTTPDelete(ctx context.Context, client *opensearch.Client, path strings.Builder) (*opensearchapi.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, path.String(), nil)
	if err != nil {
		return nil, err
	}

	if ctx != nil {
		req = req.WithContext(ctx)
	}

	res, err := client.Perform(req)
	if err != nil {
		return nil, err
	}

	return &opensearchapi.Response{StatusCode: res.StatusCode, Body: res.Body, Header: res.Header}, nil
}
