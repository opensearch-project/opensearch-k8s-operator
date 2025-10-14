// Copyright © 2019 Banzai Cloud
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
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestAnnotationRemovedWhenEmpty(t *testing.T) {
	u := unstructured.Unstructured{}
	u.SetAnnotations(map[string]string{
		LastAppliedConfig: "{}",
	})
	modified, err := DefaultAnnotator.GetModifiedConfiguration(&u, false)
	if err != nil {
		t.Fatal(err)
	}
	if "{\"metadata\":{}}" != string(modified) {
		t.Fatalf("Expected {\"metadata\":{} got %s", string(modified))
	}
}
