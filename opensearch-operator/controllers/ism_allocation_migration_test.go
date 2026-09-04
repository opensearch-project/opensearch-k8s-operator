package controllers

import (
	"encoding/json"
	"testing"
)

func TestConvertISMPolicyAllocationJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name: "legacy string fields",
			input: `{
				"spec": {
					"states": [{
						"name": "hot",
						"actions": [{
							"allocation": {
								"exclude": "box_type:cold",
								"include": "box_type:warm",
								"require": "box_type:hot",
								"waitFor": "true"
							}
						}]
					}]
				}
			}`,
			want: `{
				"spec": {
					"states": [{
						"name": "hot",
						"actions": [{
							"allocation": {
								"exclude": {"box_type": "cold"},
								"include": {"box_type": "warm"},
								"require": {"box_type": "hot"},
								"waitFor": true
							}
						}]
					}]
				}
			}`,
		},
		{
			name: "comma-separated attributes and empty strings",
			input: `{
				"spec": {
					"states": [{
						"actions": [{
							"allocation": {
								"exclude": "box_type:cold,temp:low",
								"include": "",
								"require": "box_type:hot",
								"waitFor": "false"
							}
						}]
					}]
				}
			}`,
			want: `{
				"spec": {
					"states": [{
						"actions": [{
							"allocation": {
								"exclude": {"box_type": "cold", "temp": "low"},
								"require": {"box_type": "hot"},
								"waitFor": false
							}
						}]
					}]
				}
			}`,
		},
		{
			name: "already converted map and bool fields",
			input: `{
				"spec": {
					"states": [{
						"actions": [{
							"allocation": {
								"require": {"box_type": "hot"},
								"waitFor": true
							}
						}]
					}]
				}
			}`,
			want: `{
				"spec": {
					"states": [{
						"actions": [{
							"allocation": {
								"require": {"box_type": "hot"},
								"waitFor": true
							}
						}]
					}]
				}
			}`,
		},
		{
			name: "no allocation action is left unchanged",
			input: `{
				"spec": {
					"states": [{
						"actions": [{"delete": {}}]
					}]
				}
			}`,
			want: `{
				"spec": {
					"states": [{
						"actions": [{"delete": {}}]
					}]
				}
			}`,
		},
		{
			name:    "invalid waitFor string",
			input:   `{"spec":{"states":[{"actions":[{"allocation":{"waitFor":"yes-please"}}]}]}}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertISMPolicyAllocationJSON([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("convertISMPolicyAllocationJSON() error = %v", err)
			}
			assertJSONEqual(t, tt.want, string(got))
		})
	}
}

func assertJSONEqual(t *testing.T, want, got string) {
	t.Helper()
	var wantObj, gotObj interface{}
	if err := json.Unmarshal([]byte(want), &wantObj); err != nil {
		t.Fatalf("invalid want JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(got), &gotObj); err != nil {
		t.Fatalf("invalid got JSON: %v", err)
	}
	wantNorm, _ := json.Marshal(wantObj)
	gotNorm, _ := json.Marshal(gotObj)
	if string(wantNorm) != string(gotNorm) {
		t.Errorf("JSON mismatch\nwant: %s\ngot:  %s", wantNorm, gotNorm)
	}
}
