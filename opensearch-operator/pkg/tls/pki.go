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
	"math/big"
	"time"
)

// Implementation based on https://github.com/rancher-sandbox/opni-opensearch-operator/blob/main/pkg/resources/opensearch/certs/certs.go
//  and https://github.com/rancher-sandbox/opni-opensearch-operator/blob/main/pkg/pki/pki.go

// Represents a certificate with key
type Cert struct {
	certBytes []byte
	keyBytes  []byte
}

func GenerateCA(name string) (ca Cert, err error) {
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

	return Cert{certBytes: caPEM.Bytes(), keyBytes: caKeyPEM.Bytes()}, nil
}

func (cert *Cert) Cert() (tls.Certificate, error) {
	return tls.X509KeyPair(cert.certBytes, cert.keyBytes)
}

func (ca *Cert) SecretDataCA() map[string][]byte {
	data := make(map[string][]byte)
	data["ca.crt"] = ca.certBytes
	data["ca.key"] = ca.keyBytes
	return data
}

func (cert *Cert) SecretData(ca *Cert) map[string][]byte {
	data := make(map[string][]byte)
	data["tls.crt"] = cert.certBytes
	data["tls.key"] = cert.keyBytes
	data["ca.crt"] = ca.certBytes
	return data
}

func (ca *Cert) CreateAndSignCertificate(commonName string, dnsnames []string) (cert Cert, err error) {
	tlscacert, err := ca.Cert()
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
	san, err := calculateExtension(commonName, dnsnames)
	if err != nil {
		return
	}
	x509cert := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtraExtensions: []pkix.Extension{
			san,
		},
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

	return Cert{keyBytes: keyBytes, certBytes: certBytes}, nil
}

func CAFromSecret(data map[string][]byte) Cert {
	return Cert{certBytes: data["ca.crt"], keyBytes: data["ca.key"]}
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
