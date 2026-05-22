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
	"encoding/json"
)

// isStatusOnlyUpdate checks if only the status field changed
// by comparing the specs of old and new objects
func isStatusOnlyUpdate(oldSpec, newSpec interface{}) bool {
	// Compare specs using JSON marshaling
	// If specs are equal, only status changed
	oldSpecBytes, _ := json.Marshal(oldSpec)
	newSpecBytes, _ := json.Marshal(newSpec)
	return string(oldSpecBytes) == string(newSpecBytes)
}
