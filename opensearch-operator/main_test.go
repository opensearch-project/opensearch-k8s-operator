package main

import (
	"flag"
	"testing"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
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

func TestRegisterLeaderElectionFlagsDefaults(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	leaseDuration, renewDeadline, retryPeriod := registerLeaderElectionFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if *leaseDuration != 60*time.Second {
		t.Fatalf("expected default lease duration 60s, got %s", *leaseDuration)
	}
	if *renewDeadline != 30*time.Second {
		t.Fatalf("expected default renew deadline 30s, got %s", *renewDeadline)
	}
	if *retryPeriod != 5*time.Second {
		t.Fatalf("expected default retry period 5s, got %s", *retryPeriod)
	}

	opts := ctrl.Options{
		LeaseDuration: leaseDuration,
		RenewDeadline: renewDeadline,
		RetryPeriod:   retryPeriod,
	}
	if opts.LeaseDuration == nil || *opts.LeaseDuration != 60*time.Second {
		t.Fatalf("expected ctrl.Options.LeaseDuration to be 60s, got %v", opts.LeaseDuration)
	}
	if opts.RenewDeadline == nil || *opts.RenewDeadline != 30*time.Second {
		t.Fatalf("expected ctrl.Options.RenewDeadline to be 30s, got %v", opts.RenewDeadline)
	}
	if opts.RetryPeriod == nil || *opts.RetryPeriod != 5*time.Second {
		t.Fatalf("expected ctrl.Options.RetryPeriod to be 5s, got %v", opts.RetryPeriod)
	}
}

func TestRegisterLeaderElectionFlagsOverride(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	leaseDuration, renewDeadline, retryPeriod := registerLeaderElectionFlags(fs)
	err := fs.Parse([]string{
		"-leader-elect-lease-duration=90s",
		"-leader-elect-renew-deadline=45s",
		"-leader-elect-retry-period=10s",
	})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if *leaseDuration != 90*time.Second {
		t.Fatalf("expected lease duration 90s, got %s", *leaseDuration)
	}
	if *renewDeadline != 45*time.Second {
		t.Fatalf("expected renew deadline 45s, got %s", *renewDeadline)
	}
	if *retryPeriod != 10*time.Second {
		t.Fatalf("expected retry period 10s, got %s", *retryPeriod)
	}
}
