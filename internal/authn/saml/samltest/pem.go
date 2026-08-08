package samltest

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

// writePEM writes a key pair to the two files a deployment is configured with:
// a certificate and a private key, in PEM, with the key readable only by its
// owner — which internal/config insists on and this has to satisfy.
func writePEM(t *testing.T, dir string, key *rsa.PrivateKey, certificate *x509.Certificate) (certFile, keyFile string) {
	t.Helper()

	certFile = filepath.Join(dir, "saml.crt")
	keyFile = filepath.Join(dir, "saml.key")

	write(t, certFile, 0o644, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificate.Raw,
	})
	write(t, keyFile, 0o600, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return certFile, keyFile
}

func write(t *testing.T, path string, mode os.FileMode, block *pem.Block) {
	t.Helper()

	if err := os.WriteFile(path, pem.EncodeToMemory(block), mode); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	// WriteFile applies the process umask to the mode, which on a developer's
	// machine is usually harmless and in a container is whatever the base image
	// set. Chmod is what actually makes the key file 0600, which
	// config.checkFilePrivate refuses to start without.
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("setting the mode on %s: %v", path, err)
	}
}
