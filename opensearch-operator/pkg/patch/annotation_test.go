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
	if string(modified) != "{\"metadata\":{}}" {
		t.Fatalf("Expected {\"metadata\":{} got %s", string(modified))
	}
}

func TestSetOriginalConfigurationSkipsOversizedAnnotation(t *testing.T) {
	u := unstructured.Unstructured{}
	u.SetName("test")
	u.SetNamespace("default")

	// Create data that will exceed MaxAnnotationSize after encoding
	// We need data large enough that even after zip compression it exceeds the limit
	largeData := make([]byte, MaxAnnotationSize+1024)
	for i := range largeData {
		// Use random-ish pattern to prevent good compression
		largeData[i] = byte(i % 256)
	}

	// Set the oversized configuration - should succeed without error
	err := DefaultAnnotator.SetOriginalConfiguration(&u, largeData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify no annotation was set
	annots := u.GetAnnotations()
	if annots != nil {
		if _, ok := annots[LastAppliedConfig]; ok {
			t.Fatal("Expected annotation to be skipped for oversized data")
		}
	}
}

func TestSetOriginalConfigurationSetsNormalSizedAnnotation(t *testing.T) {
	u := unstructured.Unstructured{}
	u.SetName("test")
	u.SetNamespace("default")

	// Normal sized data should be stored
	normalData := []byte(`{"apiVersion":"v1","kind":"Secret","metadata":{"name":"test"}}`)

	err := DefaultAnnotator.SetOriginalConfiguration(&u, normalData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify annotation was set
	annots := u.GetAnnotations()
	if annots == nil {
		t.Fatal("Expected annotations to be set")
	}
	if _, ok := annots[LastAppliedConfig]; !ok {
		t.Fatal("Expected last-applied annotation to be set for normal sized data")
	}

	// Verify we can retrieve the original configuration
	retrieved, err := DefaultAnnotator.GetOriginalConfiguration(&u)
	if err != nil {
		t.Fatalf("Expected no error retrieving configuration, got: %v", err)
	}
	if string(retrieved) != string(normalData) {
		t.Fatalf("Expected retrieved data to match original, got: %s", string(retrieved))
	}
}
