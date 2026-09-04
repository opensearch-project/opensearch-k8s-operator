package controllers

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	OpensearchFinalizer = "opensearch.org/opensearch-data"
)

// Controller names for per-controller concurrency configuration.
const (
	ControllerNameCluster           = "opensearchcluster"
	ControllerNameUser              = "opensearchuser"
	ControllerNameRole              = "opensearchrole"
	ControllerNameTenant            = "opensearchtenant"
	ControllerNameUserRoleBinding   = "opensearchuserrolebinding"
	ControllerNameActionGroup       = "opensearchactiongroup"
	ControllerNameISMPolicy         = "opensearchismpolicy"
	ControllerNameIndexTemplate     = "opensearchindextemplate"
	ControllerNameComponentTemplate = "opensearchcomponenttemplate"
	ControllerNameSnapshotPolicy    = "opensearchsnapshotpolicy"
)

// knownControllers is the set of controller names accepted by --max-concurrent-reconciles-per-controller.
var knownControllers = map[string]struct{}{
	ControllerNameCluster:           {},
	ControllerNameUser:              {},
	ControllerNameRole:              {},
	ControllerNameTenant:            {},
	ControllerNameUserRoleBinding:   {},
	ControllerNameActionGroup:       {},
	ControllerNameISMPolicy:         {},
	ControllerNameIndexTemplate:     {},
	ControllerNameComponentTemplate: {},
	ControllerNameSnapshotPolicy:    {},
}

// ControllerConcurrencyConfig holds concurrency settings for controllers.
type ControllerConcurrencyConfig struct {
	// Global default max concurrent reconciles for all controllers.
	MaxConcurrentReconciles int
	// Per-controller overrides (controller name -> max concurrent reconciles).
	PerController map[string]int
}

// GetMaxConcurrentReconciles returns the max concurrent reconciles for a given controller.
// Per-controller overrides take precedence over the global default. Values below 1 are clamped to 1.
func (c *ControllerConcurrencyConfig) GetMaxConcurrentReconciles(controllerName string) int {
	if c == nil {
		return 1
	}
	n := c.MaxConcurrentReconciles
	if override, exists := c.PerController[controllerName]; exists {
		n = override
	}
	if n < 1 {
		return 1
	}
	return n
}

// ParsePerControllerConcurrency parses a comma-separated list of controller=N pairs.
// Empty entries (e.g. trailing commas) are ignored. Unknown controller names,
// malformed pairs, and non-positive or unparsable values return an error.
func ParsePerControllerConcurrency(spec string) (map[string]int, error) {
	perController := make(map[string]int)
	if spec == "" {
		return perController, nil
	}
	for _, pair := range strings.Split(spec, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid per-controller concurrency entry %q: expected controller=N", pair)
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		if key == "" || val == "" {
			return nil, fmt.Errorf("invalid per-controller concurrency entry %q: expected controller=N", pair)
		}
		if _, ok := knownControllers[key]; !ok {
			return nil, fmt.Errorf("unknown controller %q in per-controller concurrency config", key)
		}
		count, err := strconv.Atoi(val)
		if err != nil {
			return nil, fmt.Errorf("invalid concurrency value %q for controller %q: %w", val, key, err)
		}
		if count < 1 {
			return nil, fmt.Errorf("concurrency for controller %q must be >= 1, got %d", key, count)
		}
		perController[key] = count
	}
	return perController, nil
}
