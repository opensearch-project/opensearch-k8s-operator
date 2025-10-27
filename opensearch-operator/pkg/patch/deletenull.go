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
	"reflect"

	"emperror.dev/errors"
	json "github.com/json-iterator/go"
)

type CalculateOption func([]byte, []byte) ([]byte, []byte, error)

func IgnoreStatusFields() CalculateOption {
	return func(current, modified []byte) ([]byte, []byte, error) {
		current, err := deleteStatusField(current)
		if err != nil {
			return []byte{}, []byte{}, errors.Wrap(err, "could not delete status field from current byte sequence")
		}

		modified, err = deleteStatusField(modified)
		if err != nil {
			return []byte{}, []byte{}, errors.Wrap(err, "could not delete status field from modified byte sequence")
		}

		return current, modified, nil
	}
}

func IgnoreVolumeClaimTemplateTypeMetaAndStatus() CalculateOption {
	return func(current, modified []byte) ([]byte, []byte, error) {
		current, err := deleteVolumeClaimTemplateFields(current)
		if err != nil {
			return []byte{}, []byte{}, errors.Wrap(err, "could not delete status field from current byte sequence")
		}

		modified, err = deleteVolumeClaimTemplateFields(modified)
		if err != nil {
			return []byte{}, []byte{}, errors.Wrap(err, "could not delete status field from modified byte sequence")
		}

		return current, modified, nil
	}
}

func DeleteNullInJson(jsonBytes []byte) ([]byte, map[string]interface{}, error) {
	var patchMap map[string]interface{}

	err := json.Unmarshal(jsonBytes, &patchMap)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not unmarshal json patch")
	}

	filteredMap, err := deleteNullInObj(patchMap)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not delete null values from patch map")
	}

	o, err := json.ConfigCompatibleWithStandardLibrary.Marshal(filteredMap)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not marshal filtered patch map")
	}

	return o, filteredMap, err
}

func deleteNullInObj(m map[string]interface{}) (map[string]interface{}, error) {
	var err error
	filteredMap := make(map[string]interface{})

	for key, val := range m {
		if val == nil || isZero(reflect.ValueOf(val)) {
			continue
		}
		switch typedVal := val.(type) {
		default:
			return nil, errors.Errorf("unknown type: %v", reflect.TypeOf(typedVal))
		case []interface{}:
			slice, err := deleteNullInSlice(typedVal)
			if err != nil {
				return nil, errors.Errorf("could not delete null values from subslice")
			}
			filteredMap[key] = slice
		case string, float64, bool, int64, nil:
			filteredMap[key] = val
		case map[string]interface{}:
			if len(typedVal) == 0 {
				filteredMap[key] = typedVal
				continue
			}

			var filteredSubMap map[string]interface{}
			filteredSubMap, err = deleteNullInObj(typedVal)
			if err != nil {
				return nil, errors.Wrap(err, "could not delete null values from filtered sub map")
			}

			if len(filteredSubMap) != 0 {
				filteredMap[key] = filteredSubMap
			}
		}
	}
	return filteredMap, nil
}

func deleteNullInSlice(m []interface{}) ([]interface{}, error) {
	filteredSlice := make([]interface{}, len(m))
	for key, val := range m {
		if val == nil {
			continue
		}
		switch typedVal := val.(type) {
		default:
			return nil, errors.Errorf("unknown type: %v", reflect.TypeOf(typedVal))
		case []interface{}:
			filteredSubSlice, err := deleteNullInSlice(typedVal)
			if err != nil {
				return nil, errors.Errorf("could not delete null values from subslice")
			}
			filteredSlice[key] = filteredSubSlice
		case string, float64, bool, int64, nil:
			filteredSlice[key] = val
		case map[string]interface{}:
			filteredMap, err := deleteNullInObj(typedVal)
			if err != nil {
				return nil, errors.Wrap(err, "could not delete null values from filtered sub map")
			}
			filteredSlice[key] = filteredMap
		}
	}
	return filteredSlice, nil
}

func deleteStatusField(obj []byte) ([]byte, error) {
	var objectMap map[string]interface{}
	err := json.Unmarshal(obj, &objectMap)
	if err != nil {
		return []byte{}, errors.Wrap(err, "could not unmarshal byte sequence")
	}
	delete(objectMap, "status")
	obj, err = json.ConfigCompatibleWithStandardLibrary.Marshal(objectMap)
	if err != nil {
		return []byte{}, errors.Wrap(err, "could not marshal byte sequence")
	}

	return obj, nil
}

func deleteVolumeClaimTemplateFields(obj []byte) ([]byte, error) {
	resource := map[string]interface{}{}
	err := json.Unmarshal(obj, &resource)
	if err != nil {
		return []byte{}, errors.Wrap(err, "could not unmarshal byte sequence")
	}

	if spec, ok := resource["spec"]; ok {
		if spec, ok := spec.(map[string]interface{}); ok {
			if vcts, ok := spec["volumeClaimTemplates"]; ok {
				if vcts, ok := vcts.([]interface{}); ok {
					for _, vct := range vcts {
						if vct, ok := vct.(map[string]interface{}); ok {
							vct["kind"] = ""
							vct["apiVersion"] = ""
							vct["status"] = map[string]string{
								"phase": "Pending",
							}
						}
					}
				}
			}
		}
	}

	obj, err = json.ConfigCompatibleWithStandardLibrary.Marshal(resource)
	if err != nil {
		return []byte{}, errors.Wrap(err, "could not marshal byte sequence")
	}

	return obj, nil
}

func isZero(v reflect.Value) bool {
	switch v.Kind() {
	default:
		z := reflect.Zero(v.Type())
		return v.Interface() == z.Interface()
	case reflect.Float64, reflect.Int64, reflect.Bool:
		return false
	case reflect.Func, reflect.Map, reflect.Slice:
		return v.IsNil()
	case reflect.Array:
		z := true
		for i := 0; i < v.Len(); i++ {
			z = z && isZero(v.Index(i))
		}
		return z
	case reflect.Struct:
		z := true
		for i := 0; i < v.NumField(); i++ {
			z = z && isZero(v.Field(i))
		}
		return z
	}
}
