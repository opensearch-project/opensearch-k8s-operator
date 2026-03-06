package controllers

const (
	OpensearchFinalizer = "opensearch.org/opensearch-data"
)

// ControllerConcurrencyConfig holds concurrency settings for controllers
type ControllerConcurrencyConfig struct {
	// Global default max concurrent reconciles for all controllers
	MaxConcurrentReconciles int
	// Per-controller overrides (controller name -> max concurrent reconciles)
	PerController map[string]int
}

// GetMaxConcurrentReconciles returns the max concurrent reconciles for a given controller
func (c *ControllerConcurrencyConfig) GetMaxConcurrentReconciles(controllerName string) int {
	n := c.MaxConcurrentReconciles
	if override, exists := c.PerController[controllerName]; exists {
		n = override
	}
	if n < 0 {
		return 1
	}
	return n
}

// Controller names for per-controller configuration
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
