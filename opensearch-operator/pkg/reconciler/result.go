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

package reconciler

import (
	"emperror.dev/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Collects results and errors of all subcomponents instead of failing and bailing out immediately
type CombinedResult struct {
	Result reconcile.Result
	Err    error
}

func (c *CombinedResult) Combine(sub *reconcile.Result, err error) {
	c.CombineErr(err)
	if sub != nil {
		if sub.Requeue {
			c.Result.Requeue = true
		}
		// combined should be requeued at the minimum of all subresults
		if sub.RequeueAfter > 0 {
			if c.Result.RequeueAfter == 0 || sub.RequeueAfter < c.Result.RequeueAfter {
				c.Result.RequeueAfter = sub.RequeueAfter
			}
		}
	}
}

func (c *CombinedResult) CombineErr(err error) {
	c.Err = errors.Combine(c.Err, err)
}
