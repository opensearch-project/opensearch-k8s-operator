package services

import (
	"bytes"
	"io"
	"net/http"
)

// contentLengthTransport buffers request bodies of unknown length and sends
// them with a Content-Length. opensearch-go never sets one, so bodies would
// otherwise be chunked, which the transport-reactor-netty4 HTTP transport
// (OpenSearch 3.0 to 3.4) never answers.
type contentLengthTransport struct {
	next http.RoundTripper
}

func (t contentLengthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body == nil || req.Body == http.NoBody || req.ContentLength > 0 {
		return t.next.RoundTrip(req)
	}

	body, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return nil, err
	}

	// RoundTrippers must not modify the request they are given.
	req = req.Clone(req.Context())
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	return t.next.RoundTrip(req)
}
