package signer_test

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/typedduck/nestor/controller/packager"
	"github.com/typedduck/nestor/controller/signer"
)

// Package-level keys generated once to avoid repeated RSA key generation cost.
var (
	sharedRSAKey     *rsa.PrivateKey
	sharedED25519Key ed25519.PrivateKey
	sharedED25519Pub ed25519.PublicKey
)

func init() {
	var err error
	sharedRSAKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic("failed to generate RSA key: " + err.Error())
	}
	sharedED25519Pub, sharedED25519Key, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic("failed to generate Ed25519 key: " + err.Error())
	}
}

// --- key-writing helpers ---

// writePKCS1Key writes an RSA private key in PKCS#1 PEM format and returns the path.
func writePKCS1Key(t *testing.T, dir string, key *rsa.PrivateKey) string {
	t.Helper()
	path := filepath.Join(dir, "key_pkcs1.pem")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}); err != nil {
		t.Fatal(err)
	}
	return path
}

// writePKCS8Key writes a private key in PKCS#8 PEM format and returns the path.
func writePKCS8Key(t *testing.T, dir string, key crypto.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal PKCS8: %v", err)
	}
	path := filepath.Join(dir, "key_pkcs8.pem")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: "PRIVATE KEY", Bytes: der}); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeOpenSSHKey writes a private key in OpenSSH PEM format and returns the path.
func writeOpenSSHKey(t *testing.T, dir string, key crypto.PrivateKey) string {
	t.Helper()
	block, err := ssh.MarshalPrivateKey(key, "")
	if err != nil {
		t.Fatalf("marshal OpenSSH key: %v", err)
	}
	path := filepath.Join(dir, "key_openssh")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// --- signing helpers ---

// fakeArchive writes arbitrary bytes to a file that acts as the archive, and
// returns a *packager.Package with ArchivePath and SignaturePath set.
func fakeArchive(t *testing.T, dir string, content []byte) *packager.Package {
	t.Helper()
	archivePath := filepath.Join(dir, "playbook.tar.gz")
	if err := os.WriteFile(archivePath, content, 0644); err != nil {
		t.Fatal(err)
	}
	return &packager.Package{
		ArchivePath:   archivePath,
		SignaturePath: filepath.Join(dir, "playbook.sig"),
	}
}

// archiveHash computes the SHA256 hash of the archive file — mirrors the
// signer's internal computeFileHash.
func archiveHash(t *testing.T, archivePath string) []byte {
	t.Helper()
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	h := sha256.Sum256(data)
	return h[:]
}

// --- tests: key loading ---

func TestNew_MissingKeyFile(t *testing.T) {
	_, err := signer.New("/nonexistent/key.pem")
	if err == nil {
		t.Fatal("expected error for missing key file")
	}
}

func TestNew_InvalidPEM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pem")
	if err := os.WriteFile(path, []byte("not a pem block"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := signer.New(path)
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

func TestNew_RSAPKCS1(t *testing.T) {
	dir := t.TempDir()
	path := writePKCS1Key(t, dir, sharedRSAKey)

	s, err := signer.New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.GetKeyType() != signer.KeyTypeRSA {
		t.Errorf("expected KeyTypeRSA, got %v", s.GetKeyType())
	}
}

func TestNew_RSAPKCS8(t *testing.T) {
	dir := t.TempDir()
	path := writePKCS8Key(t, dir, sharedRSAKey)

	s, err := signer.New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.GetKeyType() != signer.KeyTypeRSA {
		t.Errorf("expected KeyTypeRSA, got %v", s.GetKeyType())
	}
}

func TestNew_RSAOpenSSH(t *testing.T) {
	dir := t.TempDir()
	path := writeOpenSSHKey(t, dir, sharedRSAKey)

	s, err := signer.New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.GetKeyType() != signer.KeyTypeRSA {
		t.Errorf("expected KeyTypeRSA, got %v", s.GetKeyType())
	}
}

func TestNew_ED25519PKCS8(t *testing.T) {
	dir := t.TempDir()
	path := writePKCS8Key(t, dir, sharedED25519Key)

	s, err := signer.New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.GetKeyType() != signer.KeyTypeED25519 {
		t.Errorf("expected KeyTypeED25519, got %v", s.GetKeyType())
	}
}

func TestNew_ED25519OpenSSH(t *testing.T) {
	dir := t.TempDir()
	path := writeOpenSSHKey(t, dir, sharedED25519Key)

	s, err := signer.New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.GetKeyType() != signer.KeyTypeED25519 {
		t.Errorf("expected KeyTypeED25519, got %v", s.GetKeyType())
	}
}

// --- tests: key type names ---

func TestGetKeyTypeName_RSA(t *testing.T) {
	dir := t.TempDir()
	s, err := signer.New(writePKCS1Key(t, dir, sharedRSAKey))
	if err != nil {
		t.Fatal(err)
	}
	if s.GetKeyTypeName() != "RSA" {
		t.Errorf("expected RSA, got %q", s.GetKeyTypeName())
	}
}

func TestGetKeyTypeName_ED25519(t *testing.T) {
	dir := t.TempDir()
	s, err := signer.New(writePKCS8Key(t, dir, sharedED25519Key))
	if err != nil {
		t.Fatal(err)
	}
	if s.GetKeyTypeName() != "Ed25519" {
		t.Errorf("expected Ed25519, got %q", s.GetKeyTypeName())
	}
}

// --- tests: signing ---

func TestSign_ED25519_WritesSignatureFile(t *testing.T) {
	dir := t.TempDir()
	s, err := signer.New(writePKCS8Key(t, dir, sharedED25519Key))
	if err != nil {
		t.Fatal(err)
	}

	pkg := fakeArchive(t, dir, []byte("fake archive content"))
	if err := s.Sign(pkg); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if _, err := os.Stat(pkg.SignaturePath); err != nil {
		t.Errorf("signature file not created: %v", err)
	}
}

func TestSign_ED25519_SignatureVerifies(t *testing.T) {
	dir := t.TempDir()
	s, err := signer.New(writePKCS8Key(t, dir, sharedED25519Key))
	if err != nil {
		t.Fatal(err)
	}

	pkg := fakeArchive(t, dir, []byte("playbook archive bytes"))
	if err := s.Sign(pkg); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	sig, err := os.ReadFile(pkg.SignaturePath)
	if err != nil {
		t.Fatalf("read signature: %v", err)
	}

	hash := archiveHash(t, pkg.ArchivePath)
	if !ed25519.Verify(sharedED25519Pub, hash, sig) {
		t.Error("Ed25519 signature verification failed")
	}
}

func TestSign_RSA_WritesSignatureFile(t *testing.T) {
	dir := t.TempDir()
	s, err := signer.New(writePKCS1Key(t, dir, sharedRSAKey))
	if err != nil {
		t.Fatal(err)
	}

	pkg := fakeArchive(t, dir, []byte("fake archive content"))
	if err := s.Sign(pkg); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if _, err := os.Stat(pkg.SignaturePath); err != nil {
		t.Errorf("signature file not created: %v", err)
	}
}

func TestSign_RSA_SignatureVerifies(t *testing.T) {
	dir := t.TempDir()
	s, err := signer.New(writePKCS1Key(t, dir, sharedRSAKey))
	if err != nil {
		t.Fatal(err)
	}

	pkg := fakeArchive(t, dir, []byte("playbook archive bytes"))
	if err := s.Sign(pkg); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	sig, err := os.ReadFile(pkg.SignaturePath)
	if err != nil {
		t.Fatalf("read signature: %v", err)
	}

	hash := archiveHash(t, pkg.ArchivePath)
	if err := rsa.VerifyPSS(&sharedRSAKey.PublicKey, crypto.SHA256, hash, sig, nil); err != nil {
		t.Errorf("RSA signature verification failed: %v", err)
	}
}

func TestSign_MissingArchive(t *testing.T) {
	dir := t.TempDir()
	s, err := signer.New(writePKCS8Key(t, dir, sharedED25519Key))
	if err != nil {
		t.Fatal(err)
	}

	pkg := &packager.Package{
		ArchivePath:   filepath.Join(dir, "nonexistent.tar.gz"),
		SignaturePath: filepath.Join(dir, "nonexistent.sig"),
	}
	if err := s.Sign(pkg); err == nil {
		t.Fatal("expected error for missing archive")
	}
}

func TestSign_DifferentContentDifferentSignature(t *testing.T) {
	dir := t.TempDir()
	s, err := signer.New(writePKCS8Key(t, dir, sharedED25519Key))
	if err != nil {
		t.Fatal(err)
	}

	pkg1 := fakeArchive(t, t.TempDir(), []byte("content A"))
	if err := s.Sign(pkg1); err != nil {
		t.Fatalf("Sign pkg1: %v", err)
	}

	pkg2 := fakeArchive(t, t.TempDir(), []byte("content B"))
	if err := s.Sign(pkg2); err != nil {
		t.Fatalf("Sign pkg2: %v", err)
	}

	sig1, _ := os.ReadFile(pkg1.SignaturePath)
	sig2, _ := os.ReadFile(pkg2.SignaturePath)
	if string(sig1) == string(sig2) {
		t.Error("expected different signatures for different archive content")
	}
}

// --- tests: public key export ---

func TestGetPublicKey_ED25519_IsValidAuthorizedKey(t *testing.T) {
	dir := t.TempDir()
	s, err := signer.New(writePKCS8Key(t, dir, sharedED25519Key))
	if err != nil {
		t.Fatal(err)
	}

	pubKeyStr, err := s.GetPublicKey()
	if err != nil {
		t.Fatalf("GetPublicKey: %v", err)
	}

	if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKeyStr)); err != nil {
		t.Errorf("GetPublicKey returned invalid authorized key: %v", err)
	}
}

func TestGetPublicKey_RSA_IsValidAuthorizedKey(t *testing.T) {
	dir := t.TempDir()
	s, err := signer.New(writePKCS1Key(t, dir, sharedRSAKey))
	if err != nil {
		t.Fatal(err)
	}

	pubKeyStr, err := s.GetPublicKey()
	if err != nil {
		t.Fatalf("GetPublicKey: %v", err)
	}

	if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKeyStr)); err != nil {
		t.Errorf("GetPublicKey returned invalid authorized key: %v", err)
	}
}

func TestGetPublicKey_ED25519_MatchesPrivate(t *testing.T) {
	dir := t.TempDir()
	s, err := signer.New(writePKCS8Key(t, dir, sharedED25519Key))
	if err != nil {
		t.Fatal(err)
	}

	pubKeyStr, err := s.GetPublicKey()
	if err != nil {
		t.Fatalf("GetPublicKey: %v", err)
	}

	// Parse the returned authorized key and compare with the known public key.
	parsedPub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKeyStr))
	if err != nil {
		t.Fatalf("parse authorized key: %v", err)
	}

	// Marshal both to wire format and compare bytes.
	wantPub, err := ssh.NewPublicKey(sharedED25519Pub)
	if err != nil {
		t.Fatalf("ssh.NewPublicKey: %v", err)
	}
	if string(parsedPub.Marshal()) != string(wantPub.Marshal()) {
		t.Error("GetPublicKey returned a public key that does not match the private key")
	}
}

func TestGetPublicKey_ED25519_KeyTypeInOutput(t *testing.T) {
	dir := t.TempDir()
	s, err := signer.New(writePKCS8Key(t, dir, sharedED25519Key))
	if err != nil {
		t.Fatal(err)
	}

	pubKeyStr, err := s.GetPublicKey()
	if err != nil {
		t.Fatalf("GetPublicKey: %v", err)
	}

	if !strings.HasPrefix(pubKeyStr, "ssh-ed25519 ") {
		t.Errorf("expected ssh-ed25519 prefix, got: %q", pubKeyStr[:min(30, len(pubKeyStr))])
	}
}

func TestGetPublicKey_RSA_KeyTypeInOutput(t *testing.T) {
	dir := t.TempDir()
	s, err := signer.New(writePKCS1Key(t, dir, sharedRSAKey))
	if err != nil {
		t.Fatal(err)
	}

	pubKeyStr, err := s.GetPublicKey()
	if err != nil {
		t.Fatalf("GetPublicKey: %v", err)
	}

	if !strings.HasPrefix(pubKeyStr, "ssh-rsa ") {
		t.Errorf("expected ssh-rsa prefix, got: %q", pubKeyStr[:min(30, len(pubKeyStr))])
	}
}

