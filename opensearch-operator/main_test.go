package main

import (
	"testing"
)

func TestParseWatchNamespacesSingle(t *testing.T) {
	result := parseWatchNamespaces("namespace1")
	if len(result) != 1 {
		t.Fatalf("expected 1 namespace, got %d", len(result))
	}
	if _, ok := result["namespace1"]; !ok {
		t.Fatalf("expected namespace1 to be present")
	}
}

func TestParseWatchNamespacesMultiple(t *testing.T) {
	result := parseWatchNamespaces("namespace1,namespace2")
	if len(result) != 2 {
		t.Fatalf("expected 2 namespaces, got %d", len(result))
	}
	if _, ok := result["namespace1"]; !ok {
		t.Fatalf("expected namespace1 to be present")
	}
	if _, ok := result["namespace2"]; !ok {
		t.Fatalf("expected namespace2 to be present")
	}
	if _, ok := result["namespace1,namespace2"]; ok {
		t.Fatalf("did not expect unsplit namespace key to be present")
	}
}

func TestParseWatchNamespacesTrimAndSkipEmpty(t *testing.T) {
	result := parseWatchNamespaces(" namespace1, ,namespace2 ,")
	if len(result) != 2 {
		t.Fatalf("expected 2 namespaces, got %d", len(result))
	}
	if _, ok := result["namespace1"]; !ok {
		t.Fatalf("expected namespace1 to be present")
	}
	if _, ok := result["namespace2"]; !ok {
		t.Fatalf("expected namespace2 to be present")
	}
}

func TestRegisterLegacyAPIComponents(t *testing.T) {
	tests := []struct {
		name          string
		enabled       bool
		expectedCalls int
	}{
		{
			name:          "enabled",
			enabled:       true,
			expectedCalls: 1,
		},
		{
			name:          "disabled",
			enabled:       false,
			expectedCalls: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			registerLegacyAPIComponents(test.enabled, func() {
				calls++
			})

			if calls != test.expectedCalls {
				t.Fatalf("expected %d registrations, got %d", test.expectedCalls, calls)
			}
		})
	}
}
