package controllers

import (
	"reflect"
	"testing"
)

func TestGetMaxConcurrentReconciles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		cfg            *ControllerConcurrencyConfig
		controllerName string
		want           int
	}{
		{
			name:           "nil config defaults to 1",
			cfg:            nil,
			controllerName: ControllerNameCluster,
			want:           1,
		},
		{
			name: "uses global default",
			cfg: &ControllerConcurrencyConfig{
				MaxConcurrentReconciles: 4,
			},
			controllerName: ControllerNameCluster,
			want:           4,
		},
		{
			name: "per-controller override wins",
			cfg: &ControllerConcurrencyConfig{
				MaxConcurrentReconciles: 1,
				PerController: map[string]int{
					ControllerNameCluster: 8,
					ControllerNameUser:    2,
				},
			},
			controllerName: ControllerNameCluster,
			want:           8,
		},
		{
			name: "missing override falls back to global",
			cfg: &ControllerConcurrencyConfig{
				MaxConcurrentReconciles: 3,
				PerController: map[string]int{
					ControllerNameUser: 2,
				},
			},
			controllerName: ControllerNameCluster,
			want:           3,
		},
		{
			name: "zero global is clamped to 1",
			cfg: &ControllerConcurrencyConfig{
				MaxConcurrentReconciles: 0,
			},
			controllerName: ControllerNameCluster,
			want:           1,
		},
		{
			name: "negative global is clamped to 1",
			cfg: &ControllerConcurrencyConfig{
				MaxConcurrentReconciles: -5,
			},
			controllerName: ControllerNameCluster,
			want:           1,
		},
		{
			name: "zero override is clamped to 1",
			cfg: &ControllerConcurrencyConfig{
				MaxConcurrentReconciles: 4,
				PerController: map[string]int{
					ControllerNameUser: 0,
				},
			},
			controllerName: ControllerNameUser,
			want:           1,
		},
		{
			name: "negative override is clamped to 1",
			cfg: &ControllerConcurrencyConfig{
				MaxConcurrentReconciles: 4,
				PerController: map[string]int{
					ControllerNameRole: -1,
				},
			},
			controllerName: ControllerNameRole,
			want:           1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.cfg.GetMaxConcurrentReconciles(tt.controllerName)
			if got != tt.want {
				t.Fatalf("GetMaxConcurrentReconciles(%q) = %d, want %d", tt.controllerName, got, tt.want)
			}
		})
	}
}

func TestParsePerControllerConcurrency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec string
		want map[string]int
	}{
		{
			name: "empty spec",
			spec: "",
			want: map[string]int{},
		},
		{
			name: "single pair",
			spec: "opensearchcluster=4",
			want: map[string]int{ControllerNameCluster: 4},
		},
		{
			name: "multiple pairs",
			spec: "opensearchcluster=4,opensearchuser=1",
			want: map[string]int{
				ControllerNameCluster: 4,
				ControllerNameUser:    1,
			},
		},
		{
			name: "trims whitespace and lowercases keys",
			spec: " OpenSearchCluster = 4 , OpenSearchUser=2 ",
			want: map[string]int{
				ControllerNameCluster: 4,
				ControllerNameUser:    2,
			},
		},
		{
			name: "trailing comma and empty entries are skipped",
			spec: "opensearchcluster=4,,opensearchuser=1,",
			want: map[string]int{
				ControllerNameCluster: 4,
				ControllerNameUser:    1,
			},
		},
		{
			name: "invalid entries are skipped",
			spec: "opensearchcluster=4,notapair,opensearchuser=abc,opensearchrole=0,opensearchtenant=-2,=3,opensearchismpolicy=",
			want: map[string]int{ControllerNameCluster: 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ParsePerControllerConcurrency(tt.spec)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParsePerControllerConcurrency(%q) = %#v, want %#v", tt.spec, got, tt.want)
			}
		})
	}
}
