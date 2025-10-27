// Copyright © 2021 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package patch

import (
	"reflect"
	"testing"

	json "github.com/json-iterator/go"
)

func Test_unstructuredJsonMergePatch(t *testing.T) {
	type args struct {
		original map[string]interface{}
		modified map[string]interface{}
		current  map[string]interface{}
	}
	tests := []struct {
		name      string
		args      args
		wantPatch map[string]interface{}
		wantErr   bool
	}{
		{
			name: "non-existent field not deleted",
			args: args{
				original: map[string]interface{}{
					"a": "b",
				},
				modified: map[string]interface{}{},
				current:  map[string]interface{}{},
			},
			wantPatch: map[string]interface{}{},
			wantErr:   false,
		},
		{
			name: "existent field deleted",
			args: args{
				original: map[string]interface{}{
					"a": "b",
				},
				modified: map[string]interface{}{},
				current: map[string]interface{}{
					"a": "b",
				},
			},
			wantPatch: map[string]interface{}{
				"a": nil,
			},
			wantErr: false,
		},
		{
			name: "existent field updated",
			args: args{
				modified: map[string]interface{}{
					"a": "new",
				},
				current: map[string]interface{}{
					"a": "b",
				},
			},
			wantPatch: map[string]interface{}{
				"a": "new",
			},
			wantErr: false,
		},
		{
			name: "new field added",
			args: args{
				modified: map[string]interface{}{
					"a": "b",
				},
				current: map[string]interface{}{},
			},
			wantPatch: map[string]interface{}{
				"a": "b",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DefaultPatchMaker.(*PatchMaker).unstructuredJsonMergePatch(
				mustFromUnstructured(tt.args.original),
				mustFromUnstructured(tt.args.modified),
				mustFromUnstructured(tt.args.current))
			if (err != nil) != tt.wantErr {
				t.Errorf("unstructuredJsonMergePatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(mustToUnstructured(got), tt.wantPatch) {
				t.Errorf("unstructuredJsonMergePatch() got = %v, want %v", mustToUnstructured(got), tt.wantPatch)
			}
		})
	}
}

func mustFromUnstructured(u map[string]interface{}) []byte {
	r, err := json.ConfigCompatibleWithStandardLibrary.Marshal(u)
	if err != nil {
		panic(err)
	}
	return r
}

func mustToUnstructured(data []byte) map[string]interface{} {
	m := make(map[string]interface{})
	if err := json.Unmarshal(data, &m); err != nil {
		panic(err)
	}
	return m
}
