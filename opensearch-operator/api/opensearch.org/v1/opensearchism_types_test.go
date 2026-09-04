package v1

import (
	"encoding/json"
	"strings"
	"testing"
)

// Regression: plain slice + omitempty omitted an explicitly declared empty
// transitions list, so the operator's own write-back (it adds a finalizer and
// calls Update on the whole object) silently dropped it and GitOps tooling
// reported the resource as permanently out of sync. Same class as issue #1172.
func TestState_JSONRetainsExplicitEmptyTransitions(t *testing.T) {
	state := State{
		Name:        "delete",
		Actions:     []Action{},
		Transitions: &[]Transition{},
	}

	b, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	s := string(b)
	if !strings.Contains(s, `"transitions":[]`) {
		t.Fatalf(`expected JSON to contain "transitions":[], got: %s`, s)
	}
}

func TestState_JSONOmitsUnsetTransitions(t *testing.T) {
	state := State{
		Name:    "hot",
		Actions: []Action{},
	}

	b, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	s := string(b)
	if strings.Contains(s, "transitions") {
		t.Fatalf("expected JSON to omit transitions when unset, got: %s", s)
	}
}

// A populated list must still round-trip, and an empty list must survive a
// decode/encode cycle as an empty list rather than collapsing to unset -- that
// round-trip is what the API server does on every write.
func TestState_TransitionsRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "explicit empty list survives",
			in:   `{"actions":[],"name":"delete","transitions":[]}`,
			want: `"transitions":[]`,
		},
		{
			name: "populated list survives",
			in:   `{"actions":[],"name":"hot","transitions":[{"stateName":"delete"}]}`,
			want: `"stateName":"delete"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var state State
			if err := json.Unmarshal([]byte(tt.in), &state); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			b, err := json.Marshal(state)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if !strings.Contains(string(b), tt.want) {
				t.Fatalf("expected %q in %s", tt.want, b)
			}
		})
	}
}

// An absent field must decode to nil, not to an empty list, or the distinction
// the pointer exists to preserve is lost on the first read.
func TestState_AbsentTransitionsDecodesToNil(t *testing.T) {
	var state State
	if err := json.Unmarshal([]byte(`{"actions":[],"name":"hot"}`), &state); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if state.Transitions != nil {
		t.Fatalf("expected Transitions to be nil when absent, got %#v", *state.Transitions)
	}
}
