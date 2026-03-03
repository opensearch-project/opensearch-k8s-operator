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
	if override, exists := c.PerController[controllerName]; exists {
		return override
	}
	return c.MaxConcurrentReconciles
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
