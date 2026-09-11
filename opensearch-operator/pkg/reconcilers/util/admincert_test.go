package util

import (
	cryptotls "crypto/tls"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/mocks/github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	opsterTLS "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/tls"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("loadAdminCertTLSConfig", func() {
	const (
		namespace       = "test-namespace"
		adminSecretName = "admin-cert"
		caSecretName    = "http-ca"
	)

	var (
		mockClient  *k8s.MockK8sClient
		ca          opsterTLS.Cert
		adminSecret v1.Secret
	)

	// startServer serves TLS with a certificate for another host name than the one the client dials,
	// and records the common name of the client certificate.
	startServer := func(signer opsterTLS.Cert, clientCN *string) *httptest.Server {
		serverCert, err := signer.CreateAndSignCertificate("node", "OU", []string{"other.example.com"}, time.Hour)
		Expect(err).NotTo(HaveOccurred())
		keyPair, err := cryptotls.X509KeyPair(serverCert.CertData(), serverCert.KeyData())
		Expect(err).NotTo(HaveOccurred())

		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(r.TLS.PeerCertificates) > 0 {
				*clientCN = r.TLS.PeerCertificates[0].Subject.CommonName
			}
			w.WriteHeader(http.StatusOK)
		}))
		server.TLS = &cryptotls.Config{
			Certificates: []cryptotls.Certificate{keyPair},
			ClientAuth:   cryptotls.RequireAnyClientCert,
		}
		server.StartTLS()
		DeferCleanup(server.Close)
		return server
	}

	get := func(cfg *cryptotls.Config, url string) error {
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: cfg}}
		res, err := client.Get(url)
		if err == nil {
			res.Body.Close()
		}
		return err
	}

	BeforeEach(func() {
		mockClient = k8s.NewMockK8sClient(GinkgoT())
		var err error
		ca, err = opsterTLS.NewPKI().GenerateCA("test-ca")
		Expect(err).NotTo(HaveOccurred())
		adminCert, err := ca.CreateAndSignCertificate("admin", "OU", nil, time.Hour)
		Expect(err).NotTo(HaveOccurred())
		adminSecret = v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: adminSecretName, Namespace: namespace},
			Data: map[string][]byte{
				v1.TLSCertKey:              adminCert.CertData(),
				v1.TLSPrivateKeyKey:        adminCert.KeyData(),
				v1.ServiceAccountRootCAKey: ca.CertData(),
			},
		}
	})

	It("presents the admin certificate and verifies the server against the CA without checking the host name", func() {
		mockClient.EXPECT().GetSecret(adminSecretName, namespace).Return(adminSecret, nil)
		var clientCN string
		server := startServer(ca, &clientCN)

		cfg, err := loadAdminCertTLSConfig(mockClient, namespace, adminSecretName, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(get(cfg, server.URL)).To(Succeed())
		Expect(clientCN).To(Equal("admin"))
	})

	It("rejects a server certificate signed by another CA", func() {
		mockClient.EXPECT().GetSecret(adminSecretName, namespace).Return(adminSecret, nil)
		otherCA, err := opsterTLS.NewPKI().GenerateCA("other-ca")
		Expect(err).NotTo(HaveOccurred())
		var clientCN string
		server := startServer(otherCA, &clientCN)

		cfg, err := loadAdminCertTLSConfig(mockClient, namespace, adminSecretName, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(get(cfg, server.URL)).NotTo(Succeed())
	})

	It("uses the CA of a separate CA secret", func() {
		otherCA, err := opsterTLS.NewPKI().GenerateCA("other-ca")
		Expect(err).NotTo(HaveOccurred())
		mockClient.EXPECT().GetSecret(adminSecretName, namespace).Return(adminSecret, nil)
		mockClient.EXPECT().GetSecret(caSecretName, namespace).Return(v1.Secret{
			Data: map[string][]byte{v1.ServiceAccountRootCAKey: otherCA.CertData()},
		}, nil)
		var clientCN string
		server := startServer(otherCA, &clientCN)

		cfg, err := loadAdminCertTLSConfig(mockClient, namespace, adminSecretName, caSecretName)
		Expect(err).NotTo(HaveOccurred())
		Expect(get(cfg, server.URL)).To(Succeed())
	})

	It("returns an error when no CA is available", func() {
		delete(adminSecret.Data, v1.ServiceAccountRootCAKey)
		mockClient.EXPECT().GetSecret(adminSecretName, namespace).Return(adminSecret, nil)

		_, err := loadAdminCertTLSConfig(mockClient, namespace, adminSecretName, "")
		Expect(err).To(MatchError(ContainSubstring("contains no valid")))
	})
})
