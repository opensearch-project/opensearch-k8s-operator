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
