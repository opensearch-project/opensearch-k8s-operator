package services

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
)

func TestGetVotingConfigExclusions(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"none", `{}`, []string{}},
		{"empty list", `{"metadata":{"cluster_coordination":{"voting_config_exclusions":[]}}}`, []string{}},
		{
			"populated",
			`{"metadata":{"cluster_coordination":{"voting_config_exclusions":[{"node_id":"a","node_name":"masters-2"},{"node_id":"b","node_name":"masters-1"}]}}}`,
			[]string{"masters-2", "masters-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := httpmock.NewMockTransport()
			for _, m := range []string{http.MethodGet, http.MethodHead} {
				tr.RegisterResponder(m, `=~^http://os.test:9200/?$`, httpmock.NewStringResponder(200, `{"version":{"number":"2.0.0"}}`))
			}
			var gotPath, gotQuery string
			tr.RegisterResponder(http.MethodGet, `=~/_cluster/state/`, func(r *http.Request) (*http.Response, error) {
				gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
				return httpmock.NewStringResponse(200, tt.body), nil
			})
			c, err := NewOsClusterClient("http://os.test:9200", "u", "p", WithTransport(tr))
			if err != nil {
				t.Fatal(err)
			}

			got, err := c.GetVotingConfigExclusions(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
			// An index expression no index can match (names are lowercase) keeps the
			// master from copying every index's metadata into the response. A
			// leading "_" is rejected by the cluster, so it must not be used.
			segments := strings.Split(strings.TrimPrefix(gotPath, "/"), "/")
			if len(segments) != 4 || segments[0] != "_cluster" || segments[1] != "state" || segments[2] != "metadata" || segments[3] == "" || segments[3] == strings.ToLower(segments[3]) || strings.HasPrefix(segments[3], "_") {
				t.Errorf("path %q must end in a non-lowercase index name after the metadata metric", gotPath)
			}
			if !strings.Contains(gotQuery, "filter_path=metadata.cluster_coordination.voting_config_exclusions") {
				t.Errorf("query %q lacks the voting exclusions filter_path", gotQuery)
			}
		})
	}
}
