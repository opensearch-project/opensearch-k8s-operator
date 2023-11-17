package helpers

import (
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/tls"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// A simple mock to use whenever a record.EventRecorder is needed for a test
type MockEventRecorder struct{}

func (r *MockEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
}

func (r *MockEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
}

func (r *MockEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
}

func CheckVolumeExists(volumes []corev1.Volume, volumeMounts []corev1.VolumeMount, secretName string, volumeName string) bool {
	for _, volume := range volumes {
		if volume.Name == volumeName {
			for _, mount := range volumeMounts {
				if mount.Name == volumeName {
					if volume.Secret != nil {
						return volume.Secret.SecretName == secretName
					} else if volume.ConfigMap != nil {
						return volume.ConfigMap.Name == secretName
					}
				}
			}
			return false
		}
	}
	return false
}

func HasKeyWithBytes(data map[string][]byte, key string) bool {
	_, exists := data[key]
	return exists
}

type PkiMock struct{}

type CertMock struct{}

func (cert *CertMock) SecretDataCA() map[string][]byte {
	return map[string][]byte{
		"ca.crt": []byte("ca.crt"),
		"ca.key": []byte("ca.key"),
	}
}

func (cert *CertMock) SecretData(ca tls.Cert) map[string][]byte {
	return map[string][]byte{
		"ca.crt":  []byte("ca.crt"),
		"tls.key": []byte("tls.key"),
		"tls.crt": []byte("tls.crt"),
	}
}

func (cert *CertMock) KeyData() []byte {
	return []byte("tls.key")
}

func (cert *CertMock) CertData() []byte {
	return []byte("tls.crt")
}

func (ca *CertMock) CreateAndSignCertificate(commonName string, orgUnit string, dnsnames []string) (cert tls.Cert, err error) {
	return &CertMock{}, nil
}

func (pki *PkiMock) GenerateCA(name string) (ca tls.Cert, err error) {
	return &CertMock{}, nil
}

func (pki *PkiMock) CAFromSecret(data map[string][]byte) tls.Cert {
	return &CertMock{}
}

func NewMockPKI() tls.PKI {
	return &PkiMock{}
}
