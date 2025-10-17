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

package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	BanzaiCloudManagedComponent = "banzaicloud.io/managed-component"
)

// +kubebuilder:object:generate=true

// Deprecated
// Consider using ObjectMeta in the typeoverrides package combined with the merge package
type MetaBase struct {
	Annotations map[string]string `json:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

func (base *MetaBase) Merge(meta metav1.ObjectMeta) metav1.ObjectMeta {
	if base == nil {
		return meta
	}
	if len(base.Annotations) > 0 {
		if meta.Annotations == nil {
			meta.Annotations = make(map[string]string)
		}
		for key, val := range base.Annotations {
			meta.Annotations[key] = val
		}
	}
	if len(base.Labels) > 0 {
		if meta.Labels == nil {
			meta.Labels = make(map[string]string)
		}
		for key, val := range base.Labels {
			meta.Labels[key] = val
		}
	}
	return meta
}
