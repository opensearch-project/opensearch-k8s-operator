package helpers

import (
	"fmt"
	"path"
	"strings"

	"github.com/hashicorp/go-version"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/v1"
	"k8s.io/utils/pointer"
)

func ResolveInitHelperImage(cr *opensearchv1.OpenSearchCluster) (result opensearchv1.ImageSpec) {
	defaultRepo := "public.ecr.aws/opsterio"
	defaultImage := "busybox"
	defaultVersion := "1.27.2-buildx"

	// If a custom InitHelper image is specified, use it.
	if cr.Spec.InitHelper.ImageSpec != nil {
		if useCustomImage(cr.Spec.InitHelper.ImageSpec, &result) {
			return
		}
	}

	// If a different image repo is requested, use that with the default image name and version tag.
	if cr.Spec.General.DefaultRepo != nil {
		defaultRepo = *cr.Spec.General.DefaultRepo
	}

	if cr.Spec.InitHelper.Version != nil {
		defaultVersion = *cr.Spec.InitHelper.Version
	}

	result.Image = pointer.String(fmt.Sprintf("%s:%s",
		path.Join(defaultRepo, defaultImage), defaultVersion))
	return
}

func ResolveImage(cr *opensearchv1.OpenSearchCluster, nodePool *opensearchv1.NodePool) (result opensearchv1.ImageSpec) {
	defaultRepo := "docker.io/opensearchproject"
	defaultImage := "opensearch"

	// If a general custom image is specified, use it.
	if cr.Spec.General.ImageSpec != nil {
		if useCustomImage(cr.Spec.General.ImageSpec, &result) {
			return
		}
	}

	// Default to version from spec
	version := cr.Spec.General.Version

	// If a different image repo is requested, use that with the default image
	// name and version tag.
	if cr.Spec.General.DefaultRepo != nil {
		defaultRepo = *cr.Spec.General.DefaultRepo
	}

	result.Image = pointer.String(fmt.Sprintf("%s:%s",
		path.Join(defaultRepo, defaultImage), version))
	return
}

func ResolveDashboardsImage(cr *opensearchv1.OpenSearchCluster) (result opensearchv1.ImageSpec) {
	defaultRepo := "docker.io/opensearchproject"
	defaultImage := "opensearch-dashboards"

	// If a custom dashboard image is specified, use it.
	if cr.Spec.Dashboards.ImageSpec != nil {
		if useCustomImage(cr.Spec.Dashboards.ImageSpec, &result) {
			return
		}
	}

	// If a different image repo is requested, use that with the default image
	// name and version tag.
	if cr.Spec.General.DefaultRepo != nil {
		defaultRepo = *cr.Spec.General.DefaultRepo
	}

	result.Image = pointer.String(fmt.Sprintf("%s:%s",
		path.Join(defaultRepo, defaultImage), cr.Spec.Dashboards.Version))
	return
}

func useCustomImage(customImageSpec *opensearchv1.ImageSpec, result *opensearchv1.ImageSpec) bool {
	if customImageSpec != nil {
		if customImageSpec.ImagePullPolicy != nil {
			result.ImagePullPolicy = customImageSpec.ImagePullPolicy
		}
		if len(customImageSpec.ImagePullSecrets) > 0 {
			result.ImagePullSecrets = customImageSpec.ImagePullSecrets
		}
		if customImageSpec.Image != nil {
			// If custom image is specified, use it.
			result.Image = customImageSpec.Image
			return true
		}
	}
	return false
}

// Function to help identify httpPort, securityConfigPort and securityConfigPath for 1.x and 2.x OpenSearch Operator.
func VersionCheck(instance *opensearchv1.OpenSearchCluster) (int32, int32, string) {
	var httpPort int32
	var securityConfigPort int32
	var securityConfigPath string
	versionPassed, _ := version.NewVersion(instance.Spec.General.Version)
	constraints, _ := version.NewConstraint(">= 2.0")

	if instance.Spec.General.HttpPort > 0 {
		httpPort = instance.Spec.General.HttpPort
	} else {
		httpPort = 9200
	}

	if constraints.Check(versionPassed) {
		securityConfigPort = httpPort
		securityConfigPath = "/usr/share/opensearch/config/opensearch-security"
	} else {
		securityConfigPort = 9300
		securityConfigPath = "/usr/share/opensearch/plugins/opensearch-security/securityconfig"
	}
	return httpPort, securityConfigPort, securityConfigPath
}

func BuildMainCommand(installerBinary string, pluginsList []string, batchMode bool, entrypoint string) []string {
	var mainCommand []string
	com := installerBinary + " install"

	if batchMode {
		com = com + " --batch"
	}

	if len(pluginsList) > 0 {
		mainCommand = append(mainCommand, "/bin/bash", "-c")
		for _, plugin := range pluginsList {
			com = com + " '" + strings.Replace(plugin, "'", "\\'", -1) + "'"
		}

		com = com + " && " + entrypoint
		mainCommand = append(mainCommand, com)
	} else {
		mainCommand = []string{"/bin/bash", "-c", entrypoint}
	}

	return mainCommand
}

func BuildMainCommandOSD(installerBinary string, pluginsList []string, entrypoint string) []string {
	var mainCommand []string
	mainCommand = append(mainCommand, "/bin/bash", "-c")

	var com string
	for _, plugin := range pluginsList {
		com = com + installerBinary + " install" + " '" + strings.Replace(plugin, "'", "\\'", -1) + "'"
		com = com + " && "
	}
	com = com + entrypoint

	mainCommand = append(mainCommand, com)
	return mainCommand
}
