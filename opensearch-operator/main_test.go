package main

import (
	"flag"
	"testing"
	"time"
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
	if err := validateLeaderElectionDurations(*leaseDuration, *renewDeadline, *retryPeriod); err != nil {
		t.Fatalf("expected defaults to be valid, got %v", err)
	}
}

func TestValidateLeaderElectionDurations(t *testing.T) {
	tests := []struct {
		name                string
		lease, renew, retry time.Duration
		wantErr             bool
	}{
		{name: "controller-runtime defaults", lease: 15 * time.Second, renew: 10 * time.Second, retry: 2 * time.Second},
		{name: "renew equals lease", lease: 30 * time.Second, renew: 30 * time.Second, retry: 5 * time.Second, wantErr: true},
		{name: "renew greater than lease", lease: 30 * time.Second, renew: 45 * time.Second, retry: 5 * time.Second, wantErr: true},
		{name: "renew within jitter of retry", lease: 60 * time.Second, renew: 6 * time.Second, retry: 5 * time.Second, wantErr: true},
		{name: "zero retry", lease: 60 * time.Second, renew: 30 * time.Second, retry: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLeaderElectionDurations(tt.lease, tt.renew, tt.retry)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateLeaderElectionDurations(%s, %s, %s) error = %v, wantErr %v",
					tt.lease, tt.renew, tt.retry, err, tt.wantErr)
			}
		})
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
