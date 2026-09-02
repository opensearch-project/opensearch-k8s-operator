package controllers

import (
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
// Invalid entries are skipped.
func ParsePerControllerConcurrency(spec string) map[string]int {
	perController := make(map[string]int)
	if spec == "" {
		return perController
	}
	for _, pair := range strings.Split(spec, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		if key == "" || val == "" {
			continue
		}
		count, err := strconv.Atoi(val)
		if err != nil || count < 1 {
			continue
		}
		perController[key] = count
	}
	return perController
}
