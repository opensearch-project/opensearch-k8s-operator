// Copyright © 2020 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types_test

import (
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/types"
)

func TestMetaBaseEmptyOverrideOnEmptyObject(t *testing.T) {
	original := v1.ObjectMeta{}
	overrides := types.MetaBase{}

	result := overrides.Merge(original)

	if result.Labels != nil {
		t.Error("labels should be nil")
	}

	if result.Annotations != nil {
		t.Error("annotations should be nil")
	}
}

func TestMetaBaseOverrideOnEmptyObject(t *testing.T) {
	original := v1.ObjectMeta{}
	overrides := types.MetaBase{
		Annotations: map[string]string{
			"annotation": "a",
		},
		Labels: map[string]string{
			"label": "l",
		},
	}

	result := overrides.Merge(original)

	if result.Labels["label"] != "l" {
		t.Error("label should be set on empty objectmeta")
	}

	if result.Annotations["annotation"] != "a" {
		t.Error("annotations should be set on empty objectmeta")
	}
}

func TestMetaBaseOverrideOnExistingObject(t *testing.T) {
	original := v1.ObjectMeta{
		Annotations: map[string]string{
			"annotation": "a",
		},
		Labels: map[string]string{
			"label": "l",
		},
	}
	overrides := types.MetaBase{
		Annotations: map[string]string{
			"annotation": "a2",
		},
		Labels: map[string]string{
			"label": "l2",
		},
	}

	result := overrides.Merge(original)

	if result.Labels["label"] != "l2" {
		t.Error("label should be set on empty objectmeta")
	}

	if result.Annotations["annotation"] != "a2" {
		t.Error("annotations should be set on empty objectmeta")
	}
}
