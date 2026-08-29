package controllers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// convertISMPolicyAllocationJSON rewrites ISM allocation actions from the legacy
// string field types (opensearch.opster.io) to the OpenSearch API types used by
// opensearch.org (map[string]string and bool).
func convertISMPolicyAllocationJSON(raw []byte) ([]byte, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("unmarshal ISM policy for allocation conversion: %w", err)
	}
	spec, _ := obj["spec"].(map[string]interface{})
	if spec == nil {
		return raw, nil
	}
	if err := convertISMAllocationInStates(spec["states"]); err != nil {
		return nil, err
	}
	converted, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("marshal converted ISM policy: %w", err)
	}
	return converted, nil
}

func convertISMAllocationInStates(statesVal interface{}) error {
	states, ok := statesVal.([]interface{})
	if !ok {
		return nil
	}
	for _, stateVal := range states {
		state, ok := stateVal.(map[string]interface{})
		if !ok {
			continue
		}
		actions, ok := state["actions"].([]interface{})
		if !ok {
			continue
		}
		for _, actionVal := range actions {
			action, ok := actionVal.(map[string]interface{})
			if !ok {
				continue
			}
			alloc, ok := action["allocation"].(map[string]interface{})
			if !ok {
				continue
			}
			if err := convertAllocationAction(alloc); err != nil {
				return err
			}
		}
	}
	return nil
}

func convertAllocationAction(alloc map[string]interface{}) error {
	for _, key := range []string{"exclude", "include", "require"} {
		converted, err := convertAllocationAttribute(alloc[key])
		if err != nil {
			return fmt.Errorf("allocation %s: %w", key, err)
		}
		if converted == nil {
			delete(alloc, key)
		} else {
			alloc[key] = converted
		}
	}

	waitFor, err := convertWaitFor(alloc["waitFor"])
	if err != nil {
		return err
	}
	if waitFor == nil {
		delete(alloc, "waitFor")
	} else {
		alloc["waitFor"] = waitFor
	}
	return nil
}

func convertAllocationAttribute(v interface{}) (interface{}, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case map[string]interface{}:
		out := make(map[string]string, len(t))
		for k, val := range t {
			s, ok := val.(string)
			if !ok {
				return nil, fmt.Errorf("attribute %q has non-string value %T", k, val)
			}
			out[k] = s
		}
		if len(out) == 0 {
			return nil, nil
		}
		return out, nil
	case string:
		if strings.TrimSpace(t) == "" {
			return nil, nil
		}
		parsed := parseAllocationString(t)
		if len(parsed) == 0 {
			return nil, nil
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("unsupported type %T", v)
	}
}

func convertWaitFor(v interface{}) (interface{}, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case bool:
		return t, nil
	case string:
		if strings.TrimSpace(t) == "" {
			return nil, nil
		}
		parsed, err := strconv.ParseBool(t)
		if err != nil {
			return nil, fmt.Errorf("invalid value for waitFor: %s", t)
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("invalid type for waitFor, expected boolean or string, got %T", v)
	}
}

func parseAllocationString(s string) map[string]string {
	res := make(map[string]string)
	parts := strings.Split(s, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 2 {
			res[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			continue
		}
		if len(kv) == 1 && strings.TrimSpace(kv[0]) != "" {
			res[strings.TrimSpace(kv[0])] = ""
		}
	}
	return res
}
