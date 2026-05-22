/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opsterv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
)

var _ = Describe("isStatusOnlyUpdate", func() {
	It("should return true when only status changed", func() {
		oldSpec := opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
				Version: "2.19.4",
			},
		}
		newSpec := opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
				Version: "2.19.4",
			},
		}

		result := isStatusOnlyUpdate(oldSpec, newSpec)
		Expect(result).To(BeTrue())
	})

	It("should return false when spec changed", func() {
		oldSpec := opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
				Version: "2.19.4",
			},
		}
		newSpec := opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
				Version: "3.0.0",
			},
		}

		result := isStatusOnlyUpdate(oldSpec, newSpec)
		Expect(result).To(BeFalse())
	})

	It("should return true when specs are identical", func() {
		spec := opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
				Version: "2.19.4",
			},
			NodePools: []opsterv1.NodePool{
				{
					Component: "masters",
					Replicas:  3,
				},
			},
		}

		result := isStatusOnlyUpdate(spec, spec)
		Expect(result).To(BeTrue())
	})

	It("should return false when node pools changed", func() {
		oldSpec := opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
				Version: "2.19.4",
			},
			NodePools: []opsterv1.NodePool{
				{
					Component: "masters",
					Replicas:  3,
				},
			},
		}
		newSpec := opsterv1.ClusterSpec{
			General: opsterv1.GeneralConfig{
				Version: "2.19.4",
			},
			NodePools: []opsterv1.NodePool{
				{
					Component: "masters",
					Replicas:  5, // Changed replicas
				},
			},
		}

		result := isStatusOnlyUpdate(oldSpec, newSpec)
		Expect(result).To(BeFalse())
	})
})
