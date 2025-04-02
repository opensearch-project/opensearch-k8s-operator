package tls

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

// Implementation based on https://github.com/rancher-sandbox/opni-opensearch-operator/blob/main/pkg/resources/opensearch/certs/certs.go
//  and https://github.com/rancher-sandbox/opni-opensearch-operator/blob/main/pkg/pki/pki.go

type PKI interface {
	GenerateCA(name string) (ca Cert, err error)
	CAFromSecret(data map[string][]byte) Cert
}

type Cert interface {
	SecretDataCA() map[string][]byte
	SecretData(ca Cert) map[string][]byte
	KeyData() []byte
	CertData() []byte
	CreateAndSignCertificate(commonName string, orgUnit string, dnsnames []string) (cert Cert, err error)
	CreateAndSignCertificateWithExpiry(commonName string, orgUnit string, dnsnames []string, expiry time.Time) (cert Cert, err error)
}

type CertValidater interface {
	IsExpiringSoon() bool
	IsSignedByCA(ca Cert) (bool, error)
	DaysUntilExpiry() float64
	ExpiryDate() time.Time
}

// Dummy struct so that PKI interface can be implemented for easier mocking in tests
type PkiImpl struct {
}

func NewPKI() PKI {
	return &PkiImpl{}
}

// Represents a certificate with key
type PEMCert struct {
	certBytes []byte
	keyBytes  []byte
}

func (pki *PkiImpl) GenerateCA(name string) (ca Cert, err error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return
	}
	caCertTemplate := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: name,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, caCertTemplate, caCertTemplate, &caPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return
	}
	caPEM := new(bytes.Buffer)
	err = pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	if err != nil {
		return
	}

	caKeyPEM := new(bytes.Buffer)
	err = pem.Encode(caKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivateKey),
	})
	if err != nil {
		return
	}

	return &PEMCert{certBytes: caPEM.Bytes(), keyBytes: caKeyPEM.Bytes()}, nil
}

func (cert *PEMCert) cert() (tls.Certificate, error) {
	return tls.X509KeyPair(cert.certBytes, cert.keyBytes)
}

func (ca *PEMCert) SecretDataCA() map[string][]byte {
	data := make(map[string][]byte)
	data["ca.crt"] = ca.certBytes
	data["ca.key"] = ca.keyBytes
	return data
}

func (cert *PEMCert) SecretData(ca Cert) map[string][]byte {
	data := make(map[string][]byte)
	data["tls.crt"] = cert.certBytes
	data["tls.key"] = cert.keyBytes
	data["ca.crt"] = ca.CertData()
	return data
}

func (cert *PEMCert) KeyData() []byte {
	return cert.keyBytes
}

func (cert *PEMCert) CertData() []byte {
	return cert.certBytes
}

func (ca *PEMCert) CreateAndSignCertificateWithExpiry(commonName string, orgUnit string, dnsnames []string, expiry time.Time) (cert Cert, err error) {
	tlscacert, err := ca.cert()
	if err != nil {
		return
	}
	cacert, err := x509.ParseCertificate(tlscacert.Certificate[0])
	if err != nil {
		return
	}

	keypair, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return
	}

	x509cert := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:         commonName,
			OrganizationalUnit: []string{orgUnit},
		},
		NotBefore:   time.Now(),
		NotAfter:    expiry,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}
	if len(dnsnames) > 0 {
		san, err := calculateExtension(commonName, dnsnames)
		if err != nil {
			return cert, err
		}
		x509cert.ExtraExtensions = []pkix.Extension{san}
	}

	signed, err := x509.CreateCertificate(rand.Reader, x509cert, cacert, &keypair.PublicKey, tlscacert.PrivateKey)
	if err != nil {
		return
	}

	certPEMBuffer := new(bytes.Buffer)
	err = pem.Encode(certPEMBuffer, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: signed,
	})
	if err != nil {
		return
	}
	certBytes := certPEMBuffer.Bytes()

	pkcs8key, err := x509.MarshalPKCS8PrivateKey(keypair)
	if err != nil {
		return
	}

	keyPEM := new(bytes.Buffer)
	err = pem.Encode(keyPEM, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8key,
	})
	if err != nil {
		return
	}
	keyBytes := keyPEM.Bytes()

	return &PEMCert{keyBytes: keyBytes, certBytes: certBytes}, nil
}

func (ca *PEMCert) CreateAndSignCertificate(commonName string, orgUnit string, dnsnames []string) (cert Cert, err error) {
	return ca.CreateAndSignCertificateWithExpiry(commonName, orgUnit, dnsnames, time.Now().AddDate(1, 0, 0))
}

func (pki *PkiImpl) CAFromSecret(data map[string][]byte) Cert {
	return &PEMCert{certBytes: data["ca.crt"], keyBytes: data["ca.key"]}
}

func calculateExtension(commonName string, dnsNames []string) (pkix.Extension, error) {
	rawValues := []asn1.RawValue{
		{FullBytes: []byte{0x88, 0x05, 0x2A, 0x03, 0x04, 0x05, 0x05}},
	}
	for _, name := range dnsNames {
		rawValues = append(rawValues, asn1.RawValue{Tag: 2, Class: 2, Bytes: []byte(name)})
	}
	rawByte, err := asn1.Marshal(rawValues)
	if err != nil {
		return pkix.Extension{}, err
	}
	san := pkix.Extension{
		Id:       asn1.ObjectIdentifier{2, 5, 29, 17},
		Critical: true,
		Value:    rawByte,
	}
	return san, nil
}

type implCertValidater struct {
	implCertValidaterOptions
	cert *x509.Certificate
}

type implCertValidaterOptions struct {
	expiryThreshold time.Duration
}

type ImplCertValidaterOption func(*implCertValidaterOptions)

func (o *implCertValidaterOptions) apply(opts ...ImplCertValidaterOption) {
	for _, opt := range opts {
		opt(o)
	}
}

func WithExpiryThreshold(expiryThreshold time.Duration) ImplCertValidaterOption {
	return func(o *implCertValidaterOptions) {
		o.expiryThreshold = expiryThreshold
	}
}

func NewCertValidater(pemData []byte, opts ...ImplCertValidaterOption) (CertValidater, error) {
	var o implCertValidaterOptions
	o.apply(opts...)

	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM data")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return &implCertValidater{
		implCertValidaterOptions: o,
		cert:                     cert,
	}, nil
}

func (i *implCertValidater) IsExpiringSoon() bool {
	return time.Now().After(i.cert.NotAfter.Add(i.expiryThreshold * -1))
}

func (i *implCertValidater) IsSignedByCA(ca Cert) (bool, error) {
	block, _ := pem.Decode(ca.CertData())
	if block == nil {
		return false, fmt.Errorf("failed to decode CA certificate PEM data")
	}

	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, err
	}

	return bytes.Equal(i.cert.RawIssuer, caCert.RawSubject), nil
}

func (i *implCertValidater) DaysUntilExpiry() float64 {
	duration := time.Until(i.cert.NotAfter)
	return duration.Hours() / 24
}

func (i *implCertValidater) ExpiryDate() time.Time {
	return i.cert.NotAfter
}
