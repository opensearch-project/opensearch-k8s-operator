package services

import (
	"net/http"
	"testing"

	"github.com/go-logr/logr"
	"github.com/jarcoal/httpmock"
)

// Re-excluding a node that is already excluded republishes cluster state for
// nothing, and the restart path does it once per pass for the whole of a drain.
func TestAppendExcludeNodeHostIsIdempotent(t *testing.T) {
	tests := []struct {
		name      string
		excluded  string
		wantWrite bool
	}{
		{"an empty list is written", "", true},
		{"a node already excluded writes nothing", "n0", false},
		{"another node is appended", "n1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := httpmock.NewMockTransport()
			for _, m := range []string{http.MethodGet, http.MethodHead} {
				tr.RegisterResponder(m, `=~^http://os.test:9200/?$`, httpmock.NewStringResponder(200, `{"version":{"number":"2.0.0"}}`))
			}
			tr.RegisterResponder(http.MethodGet, `=~/_cluster/settings`, httpmock.NewStringResponder(200,
				`{"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_name":"`+tt.excluded+`"}}}}},"persistent":{}}`))
			writes := 0
			tr.RegisterResponder(http.MethodPut, `=~/_cluster/settings`, func(*http.Request) (*http.Response, error) {
				writes++
				return httpmock.NewStringResponse(200, `{}`), nil
			})
			c, err := NewOsClusterClient("http://os.test:9200", "u", "p", WithTransport(tr))
			if err != nil {
				t.Fatal(err)
			}
			ok, err := AppendExcludeNodeHost(c, logr.Discard(), "n0")
			if err != nil || !ok {
				t.Fatalf("got ok=%v err=%v", ok, err)
			}
			if (writes > 0) != tt.wantWrite {
				t.Errorf("writes=%d, wantWrite=%v", writes, tt.wantWrite)
			}
		})
	}
}
