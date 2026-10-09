package services

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/opensearch-project/opensearch-go/opensearchutil"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
)

type seenRequest struct {
	method           string
	contentLength    int64
	transferEncoding []string
	body             string
}

// writeRecorder starts a server that records every non-GET request.
func writeRecorder(t *testing.T) (*httptest.Server, func() []seenRequest) {
	t.Helper()
	var mu sync.Mutex
	var seen []seenRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			_, _ = io.WriteString(w, `{"version":{"number":"3.3.2"}}`)
			return
		}
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, seenRequest{r.Method, r.ContentLength, r.TransferEncoding, string(b)})
		mu.Unlock()
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []seenRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]seenRequest(nil), seen...)
	}
}

func assertSentWithLength(t *testing.T, seen []seenRequest, method, wantInBody string) {
	t.Helper()
	if len(seen) != 1 {
		t.Fatalf("got %d write requests, want 1", len(seen))
	}
	got := seen[0]
	if got.method != method {
		t.Errorf("method %s, want %s", got.method, method)
	}
	if got.contentLength != int64(len(got.body)) || len(got.transferEncoding) != 0 {
		t.Errorf("Content-Length %d, Transfer-Encoding %v; want %d and none", got.contentLength, got.transferEncoding, len(got.body))
	}
	if !strings.Contains(got.body, wantInBody) {
		t.Errorf("body %q does not contain %q", got.body, wantInBody)
	}
}

func TestWritesAreSentWithContentLength(t *testing.T) {
	const jsonBody = `"index_patterns":["a-*"]`
	var path strings.Builder
	path.WriteString("/_index_template/t")
	tests := []struct {
		name   string
		method string
		body   string
		write  func(c *OsClusterClient) error
	}{
		{"doHTTPPut", http.MethodPut, jsonBody, func(c *OsClusterClient) error {
			_, err := doHTTPPut(context.TODO(), c.client, path, opensearchutil.NewJSONReader(map[string]any{"index_patterns": []string{"a-*"}}))
			return err
		}},
		{"doHTTPPost", http.MethodPost, jsonBody, func(c *OsClusterClient) error {
			_, err := doHTTPPost(context.TODO(), c.client, path, opensearchutil.NewJSONReader(map[string]any{"index_patterns": []string{"a-*"}}))
			return err
		}},
		{"PutClusterSettings", http.MethodPut, `"cluster.routing.allocation.exclude._name":"node-0"`, func(c *OsClusterClient) error {
			_, err := c.PutClusterSettings(responses.ClusterSettingsResponse{
				Persistent: map[string]any{"cluster.routing.allocation.exclude._name": "node-0"},
				Transient:  map[string]any{},
			})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, seen := writeRecorder(t)
			c, err := NewOsClusterClient(srv.URL, "u", "p")
			if err != nil {
				t.Fatal(err)
			}
			if err := tt.write(c); err != nil {
				t.Fatal(err)
			}
			assertSentWithLength(t, seen(), tt.method, tt.body)
		})
	}
}

type captureTransport struct{ got *http.Request }

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.got = req
	return &http.Response{StatusCode: 200, Body: http.NoBody, Request: req}, nil
}

func TestContentLengthTransportBuffersUnknownLength(t *testing.T) {
	next := &captureTransport{}
	orig, _ := http.NewRequest(http.MethodPut, "http://os.test/x", io.NopCloser(strings.NewReader("hello")))
	if orig.ContentLength != 0 {
		t.Fatalf("test setup: ContentLength %d, want 0", orig.ContentLength)
	}
	if _, err := (contentLengthTransport{next: next}).RoundTrip(orig); err != nil {
		t.Fatal(err)
	}

	got := next.got
	if got.ContentLength != 5 {
		t.Errorf("ContentLength %d, want 5", got.ContentLength)
	}
	if b, _ := io.ReadAll(got.Body); string(b) != "hello" {
		t.Errorf("body %q, want hello", b)
	}
	for i := 0; i < 2; i++ {
		rc, err := got.GetBody()
		if err != nil {
			t.Fatal(err)
		}
		if b, _ := io.ReadAll(rc); string(b) != "hello" {
			t.Errorf("GetBody replay %d: %q, want hello", i, b)
		}
	}
	if orig.ContentLength != 0 || orig.GetBody != nil {
		t.Error("input request was modified")
	}
}

func TestContentLengthTransportPassesThrough(t *testing.T) {
	bodyReq, _ := http.NewRequest(http.MethodPut, "http://os.test/x", strings.NewReader("hello"))
	getReq, _ := http.NewRequest(http.MethodGet, "http://os.test/x", nil)
	for name, req := range map[string]*http.Request{"no body": getReq, "known length": bodyReq} {
		t.Run(name, func(t *testing.T) {
			next := &captureTransport{}
			if _, err := (contentLengthTransport{next: next}).RoundTrip(req); err != nil {
				t.Fatal(err)
			}
			if next.got != req {
				t.Error("request was cloned, want it passed through unchanged")
			}
		})
	}
}
