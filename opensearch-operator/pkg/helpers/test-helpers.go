package helpers

import (
	"time"

	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/tls"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// openssl req -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes -keyout ec_private.key -x509 -days 365 -out ec_certificate.crt
const testCert = `-----BEGIN CERTIFICATE-----
MIIB9DCCAZmgAwIBAgIULO/hHVrHV4Cy4tSRgG2pn9s84kkwCgYIKoZIzj0EAwIw
TzELMAkGA1UEBhMCSU4xCzAJBgNVBAgMAlROMQwwCgYDVQQHDANHQ0MxCzAJBgNV
BAoMAkZXMQswCQYDVQQLDAJDRTELMAkGA1UEAwwCT1MwHhcNMjUwMzI2MDgzODUx
WhcNMzIwNjI3MDgzODUxWjBPMQswCQYDVQQGEwJJTjELMAkGA1UECAwCVE4xDDAK
BgNVBAcMA0dDQzELMAkGA1UECgwCRlcxCzAJBgNVBAsMAkNFMQswCQYDVQQDDAJP
UzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABL6Cfm71Z4EEkElklTV/ZrFCXC4W
zcrtpRcJ4wRhSb6ot1AShNr+g5paRGrltZ2Qaph8MHxoMzjCw33dGz4GeYOjUzBR
MB0GA1UdDgQWBBR8lDV4Lj3wF9LpwFvpg76oumaV8TAfBgNVHSMEGDAWgBR8lDV4
Lj3wF9LpwFvpg76oumaV8TAPBgNVHRMBAf8EBTADAQH/MAoGCCqGSM49BAMCA0kA
MEYCIQDjVQcGNf1gAC45taNHDymROEpiJO3mDXddKu/5c/72BAIhAK48LT3/4L0V
bB/wGgSy0q1QWrIhfWxT8gc+qancJ/Ab
-----END CERTIFICATE-----
`

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

type PkiMock struct {
	UsedCertMockRef *CertMock
}

type CertMock struct {
	LastExpiryTime                                   time.Time
	NumTimesCalledCreateAndSignCertificate           int
	NumTimesCalledCreateAndSignCertificateWithExpiry int
}

func (cert *CertMock) SecretDataCA() map[string][]byte {
	return map[string][]byte{
		"ca.crt": []byte(testCert),
		"ca.key": []byte("ca.key"),
	}
}

func (cert *CertMock) SecretData(ca tls.Cert) map[string][]byte {
	return map[string][]byte{
		"ca.crt":  []byte(testCert),
		"tls.key": []byte("tls.key"),
		"tls.crt": []byte(testCert),
	}
}

func (cert *CertMock) KeyData() []byte {
	return []byte("tls.key")
}

func (cert *CertMock) CertData() []byte {
	return []byte(testCert)
}

func (ca *CertMock) CreateAndSignCertificate(commonName string, orgUnit string, dnsnames []string) (cert tls.Cert, err error) {
	ca.NumTimesCalledCreateAndSignCertificate += 1
	// Calling this method is equivalent to calling CreateAndSignCertificateWithExpiry
	// with the default expiry time
	ca.NumTimesCalledCreateAndSignCertificateWithExpiry += 1
	return ca, nil
}

func (ca *CertMock) CreateAndSignCertificateWithExpiry(commonName string, orgUnit string, dnsnames []string, expiry time.Time) (cert tls.Cert, err error) {
	ca.NumTimesCalledCreateAndSignCertificateWithExpiry += 1
	ca.LastExpiryTime = expiry
	return ca, nil
}

func (pki *PkiMock) GenerateCA(name string) (ca tls.Cert, err error) {
	if pki.UsedCertMockRef != nil {
		return pki.UsedCertMockRef, nil
	}
	pki.UsedCertMockRef = &CertMock{}
	return pki.UsedCertMockRef, nil
}

func (pki *PkiMock) CAFromSecret(data map[string][]byte) tls.Cert {
	if pki.UsedCertMockRef != nil {
		return pki.UsedCertMockRef
	}
	pki.UsedCertMockRef = &CertMock{}
	return pki.UsedCertMockRef
}

func NewMockPKI() tls.PKI {
	return &PkiMock{}
}

func (pki *PkiMock) GetUsedCertMock() *CertMock {
	return pki.UsedCertMockRef
}
