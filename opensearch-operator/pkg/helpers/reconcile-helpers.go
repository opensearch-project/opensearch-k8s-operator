package helpers

import (
	"fmt"
	"path"
	"strings"

	"k8s.io/utils/ptr"

	"github.com/hashicorp/go-version"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
)

func ResolveInitHelperImage(cr *opensearchv1.OpenSearchCluster) (result opensearchv1.ImageSpec) {
	defaultRepo := "docker.io"
	defaultImage := "busybox"
	defaultVersion := "latest"

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

	result.Image = ptr.To(fmt.Sprintf("%s:%s",
		path.Join(defaultRepo, defaultImage), defaultVersion))
	return
}

func ResolveImage(cr *opensearchv1.OpenSearchCluster, nodePool *opensearchv1.NodePool) (result opensearchv1.ImageSpec) {
	if cr == nil {
		return
	}

	version := cr.Spec.General.Version
	defaultRepo := "docker.io/opensearchproject"
	if cr.Spec.General.DefaultRepo != nil {
		defaultRepo = *cr.Spec.General.DefaultRepo
	}
	imageSpec := cr.Spec.General.ImageSpec

	defaultImage := "opensearch"

	// If a general custom image is specified, use it.
	if imageSpec != nil {
		if useCustomImage(imageSpec, &result) {
			return
		}
	}

	// If a different image repo is requested, use that with the default image
	// name and version tag.
	result.Image = ptr.To(fmt.Sprintf("%s:%s",
		path.Join(defaultRepo, defaultImage), version))
	return
}

func ResolveDashboardsImage(cr *opensearchv1.OpenSearchCluster) (result opensearchv1.ImageSpec) {
	if cr == nil {
		return
	}

	defaultRepo := "docker.io/opensearchproject"
	defaultImage := "opensearch-dashboards"
	version := cr.Spec.Dashboards.Version
	if version == "" {
		version = cr.Spec.General.Version
	}
	if cr.Spec.General.DefaultRepo != nil {
		defaultRepo = *cr.Spec.General.DefaultRepo
	}
	imageSpec := cr.Spec.Dashboards.ImageSpec

	// If a custom dashboard image is specified, use it.
	if imageSpec != nil {
		if useCustomImage(imageSpec, &result) {
			return
		}
	}

	result.Image = ptr.To(fmt.Sprintf("%s:%s",
		path.Join(defaultRepo, defaultImage), version))
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
	versionPassed, err := version.NewVersion(instance.Spec.General.Version)
	if err != nil {
		// If version parsing fails, default to 1.x behavior
		versionPassed = nil
	}
	constraints, _ := version.NewConstraint(">= 2.0")

	if instance.Spec.General.HttpPort > 0 {
		httpPort = instance.Spec.General.HttpPort
	} else {
		httpPort = 9200
	}

	// Check if version is >= 2.0, handling prerelease versions correctly
	// For prerelease versions like "3.0.0-testing", we need to compare the base version
	isVersion2OrHigher := false
	if versionPassed != nil {
		// Get the segments (major, minor, patch) to create a base version without prerelease
		segments := versionPassed.Segments()
		if len(segments) > 0 {
			// Create a base version string from segments (e.g., "3.0.0" from "3.0.0-testing")
			// Always include at least major.minor to ensure proper comparison with ">= 2.0"
			major := segments[0]
			minor := 0
			if len(segments) > 1 {
				minor = segments[1]
			}
			patch := 0
			if len(segments) > 2 {
				patch = segments[2]
			}
			baseVersionStr := fmt.Sprintf("%d.%d.%d", major, minor, patch)
			baseVersion, err := version.NewVersion(baseVersionStr)
			if err == nil {
				isVersion2OrHigher = constraints.Check(baseVersion)
			}
		}
	}

	if isVersion2OrHigher {
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
			com = com + " '" + strings.ReplaceAll(plugin, "'", "\\'") + "'"
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
		com = com + installerBinary + " install" + " '" + strings.ReplaceAll(plugin, "'", "\\'") + "'"
		com = com + " && "
	}
	com = com + entrypoint

	mainCommand = append(mainCommand, com)
	return mainCommand
}
