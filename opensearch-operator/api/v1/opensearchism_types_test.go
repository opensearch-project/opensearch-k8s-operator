package v1

import (
	"encoding/json"
	"testing"
)

func TestAllocationUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		expected Allocation
	}{
		{
			name:     "new map format and boolean waitFor",
			jsonData: `{"exclude":{"box_type":"cold"},"include":{"box_type":"warm"},"require":{"box_type":"hot"},"waitFor":true}`,
			expected: Allocation{
				Exclude: map[string]string{"box_type": "cold"},
				Include: map[string]string{"box_type": "warm"},
				Require: map[string]string{"box_type": "hot"},
				WaitFor: func() *bool { b := true; return &b }(),
			},
		},
		{
			name:     "old string format and string waitFor",
			jsonData: `{"exclude":"box_type:cold","include":"box_type:warm","require":"box_type:hot","waitFor":"true"}`,
			expected: Allocation{
				Exclude: map[string]string{"box_type": "cold"},
				Include: map[string]string{"box_type": "warm"},
				Require: map[string]string{"box_type": "hot"},
				WaitFor: func() *bool { b := true; return &b }(),
			},
		},
		{
			name:     "old string format with empty fields",
			jsonData: `{"exclude":"","include":"","require":"","waitFor":"false"}`,
			expected: Allocation{
				Exclude: nil,
				Include: nil,
				Require: nil,
				WaitFor: func() *bool { b := false; return &b }(),
			},
		},
		{
			name:     "old string format with multiple comma separated attributes",
			jsonData: `{"exclude":"box_type:cold,temp:low","include":"box_type:warm,temp:medium","require":"box_type:hot,temp:high","waitFor":"true"}`,
			expected: Allocation{
				Exclude: map[string]string{"box_type": "cold", "temp": "low"},
				Include: map[string]string{"box_type": "warm", "temp": "medium"},
				Require: map[string]string{"box_type": "hot", "temp": "high"},
				WaitFor: func() *bool { b := true; return &b }(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var alloc Allocation
			err := json.Unmarshal([]byte(tt.jsonData), &alloc)
			if err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}

			// Verify Exclude
			if len(alloc.Exclude) != len(tt.expected.Exclude) {
				t.Errorf("expected exclude %v, got %v", tt.expected.Exclude, alloc.Exclude)
			}
			for k, v := range tt.expected.Exclude {
				if alloc.Exclude[k] != v {
					t.Errorf("expected exclude %v, got %v", tt.expected.Exclude, alloc.Exclude)
				}
			}

			// Verify Include
			if len(alloc.Include) != len(tt.expected.Include) {
				t.Errorf("expected include %v, got %v", tt.expected.Include, alloc.Include)
			}
			for k, v := range tt.expected.Include {
				if alloc.Include[k] != v {
					t.Errorf("expected include %v, got %v", tt.expected.Include, alloc.Include)
				}
			}

			// Verify Require
			if len(alloc.Require) != len(tt.expected.Require) {
				t.Errorf("expected require %v, got %v", tt.expected.Require, alloc.Require)
			}
			for k, v := range tt.expected.Require {
				if alloc.Require[k] != v {
					t.Errorf("expected require %v, got %v", tt.expected.Require, alloc.Require)
				}
			}

			// Verify WaitFor
			if (alloc.WaitFor == nil) != (tt.expected.WaitFor == nil) {
				t.Errorf("expected waitFor nil state to be %v, got %v", tt.expected.WaitFor == nil, alloc.WaitFor == nil)
			} else if alloc.WaitFor != nil && *alloc.WaitFor != *tt.expected.WaitFor {
				t.Errorf("expected waitFor %v, got %v", *tt.expected.WaitFor, *alloc.WaitFor)
			}
		})
	}
}
