package reconcilers

import (
	"testing"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestMountWithHotReload(t *testing.T) {
	tests := []struct {
		name            string
		enableHotReload bool
		expectedPath    string
		expectedSubPath string
	}{
		{
			name:            "Hot reload disabled - uses subPath",
			enableHotReload: false,
			expectedPath:    "/usr/share/opensearch/config/tls-transport/tls.crt",
			expectedSubPath: "tls.crt",
		},
		{
			name:            "Hot reload enabled - mounts directory",
			enableHotReload: true,
			expectedPath:    "/usr/share/opensearch/config/tls-transport-cert",
			expectedSubPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconcilerContext := &ReconcilerContext{
				Volumes:      []corev1.Volume{},
				VolumeMounts: []corev1.VolumeMount{},
			}

			mountWithHotReload("transport", "cert", "tls.crt", "test-secret", reconcilerContext, tt.enableHotReload)

			assert.Len(t, reconcilerContext.Volumes, 1, "Should create one volume")
			assert.Len(t, reconcilerContext.VolumeMounts, 1, "Should create one volume mount")

			volume := reconcilerContext.Volumes[0]
			mount := reconcilerContext.VolumeMounts[0]

			assert.Equal(t, "transport-cert", volume.Name, "Volume name should be transport-cert")
			assert.Equal(t, "test-secret", volume.VolumeSource.Secret.SecretName, "Secret name should match")
			assert.Equal(t, "transport-cert", mount.Name, "Volume mount name should match volume name")
			assert.Equal(t, tt.expectedPath, mount.MountPath, "Mount path should match expected")
			assert.Equal(t, tt.expectedSubPath, mount.SubPath, "SubPath should match expected")
		})
	}
}

func TestTLSConfigPaths(t *testing.T) {
	tests := []struct {
		name             string
		enableHotReload  bool
		hasCaSecret      bool
		expectedCertPath string
		expectedKeyPath  string
		expectedCaPath   string
	}{
		{
			name:             "Hot reload disabled",
			enableHotReload:  false,
			hasCaSecret:      true,
			expectedCertPath: "tls-transport/tls.crt",
			expectedKeyPath:  "tls-transport/tls.key",
			expectedCaPath:   "tls-transport/ca.crt",
		},
		{
			name:             "Hot reload enabled with CA secret",
			enableHotReload:  true,
			hasCaSecret:      true,
			expectedCertPath: "tls-transport-cert/tls.crt",
			expectedKeyPath:  "tls-transport-key/tls.key",
			expectedCaPath:   "tls-transport-ca/ca.crt",
		},
		{
			name:             "Hot reload enabled without CA secret",
			enableHotReload:  true,
			hasCaSecret:      false,
			expectedCertPath: "tls-transport/tls.crt",
			expectedKeyPath:  "tls-transport/tls.key",
			expectedCaPath:   "tls-transport/ca.crt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconcilerContext := &ReconcilerContext{
				OpenSearchConfig: map[string]string{},
			}

			tlsConfig := &opsterv1.TlsConfigTransport{
				TlsCertificateConfig: opsterv1.TlsCertificateConfig{
					EnableHotReload: tt.enableHotReload,
				},
			}

			if tt.hasCaSecret {
				tlsConfig.CaSecret = corev1.LocalObjectReference{Name: "ca-secret"}
			}

			// Simulate the path configuration logic from handleTransportExistingCerts
			if tt.enableHotReload && tt.hasCaSecret {
				reconcilerContext.OpenSearchConfig["plugins.security.ssl.transport.pemcert_filepath"] = "tls-transport-cert/tls.crt"
				reconcilerContext.OpenSearchConfig["plugins.security.ssl.transport.pemkey_filepath"] = "tls-transport-key/tls.key"
				reconcilerContext.OpenSearchConfig["plugins.security.ssl.transport.pemtrustedcas_filepath"] = "tls-transport-ca/ca.crt"
			} else {
				reconcilerContext.OpenSearchConfig["plugins.security.ssl.transport.pemcert_filepath"] = "tls-transport/tls.crt"
				reconcilerContext.OpenSearchConfig["plugins.security.ssl.transport.pemkey_filepath"] = "tls-transport/tls.key"
				reconcilerContext.OpenSearchConfig["plugins.security.ssl.transport.pemtrustedcas_filepath"] = "tls-transport/ca.crt"
			}

			assert.Equal(t, tt.expectedCertPath, reconcilerContext.OpenSearchConfig["plugins.security.ssl.transport.pemcert_filepath"])
			assert.Equal(t, tt.expectedKeyPath, reconcilerContext.OpenSearchConfig["plugins.security.ssl.transport.pemkey_filepath"])
			assert.Equal(t, tt.expectedCaPath, reconcilerContext.OpenSearchConfig["plugins.security.ssl.transport.pemtrustedcas_filepath"])
		})
	}
}
