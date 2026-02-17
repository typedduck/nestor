package signer

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"

	"github.com/typedduck/nestor/controller/packager"
)

// KeyType represents the type of signing key
type KeyType int

const (
	KeyTypeUnknown KeyType = iota
	KeyTypeRSA
	KeyTypeED25519
)

// Signer signs playbook packages
type Signer struct {
	privateKeyPath string
	keyType        KeyType

	// One of these will be set based on keyType
	rsaKey     *rsa.PrivateKey
	ed25519Key ed25519.PrivateKey
}

// New creates a new signer with the specified private key
//
// Supports both RSA and Ed25519 keys.
// The key type is automatically detected from the key file.
func New(privateKeyPath string) (*Signer, error) {
	s := &Signer{
		privateKeyPath: privateKeyPath,
	}

	// Load and detect key type
	if err := s.loadPrivateKey(); err != nil {
		return nil, fmt.Errorf("failed to load private key: %w", err)
	}

	return s, nil
}

// loadPrivateKey loads the private key and detects its type
func (s *Signer) loadPrivateKey() error {
	// Read key file
	keyData, err := os.ReadFile(s.privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}

	// Decode PEM block
	block, _ := pem.Decode(keyData)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}

	// Try to parse as different key types

	// Try RSA PKCS1 format
	if rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		s.rsaKey = rsaKey
		s.keyType = KeyTypeRSA
		return nil
	}

	// Try PKCS8 format (supports both RSA and Ed25519)
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		switch k := key.(type) {
		case *rsa.PrivateKey:
			s.rsaKey = k
			s.keyType = KeyTypeRSA
			return nil
		case ed25519.PrivateKey:
			s.ed25519Key = k
			s.keyType = KeyTypeED25519
			return nil
		default:
			return fmt.Errorf("unsupported key type in PKCS8: %T", key)
		}
	}

	// Try OpenSSH format (used by ssh-keygen for Ed25519)
	if key, err := ssh.ParseRawPrivateKey(keyData); err == nil {
		switch k := key.(type) {
		case *rsa.PrivateKey:
			s.rsaKey = k
			s.keyType = KeyTypeRSA
			return nil
		case *ed25519.PrivateKey:
			s.ed25519Key = *k
			s.keyType = KeyTypeED25519
			return nil
		default:
			return fmt.Errorf("unsupported key type from OpenSSH: %T", key)
		}
	}

	return fmt.Errorf("failed to parse private key (tried RSA PKCS1, PKCS8, and OpenSSH formats)")
}

// Sign signs a playbook package
//
// The signature is created by:
// 1. Computing SHA256 hash of the archive
// 2. Signing the hash with the private key
//   - RSA keys use RSA-PSS
//   - Ed25519 keys use Ed25519 signature
//
// 3. Writing the signature to a file in the package directory
func (s *Signer) Sign(pkg *packager.Package) error {
	// Compute hash of the archive
	archiveHash, err := computeFileHash(pkg.ArchivePath)
	if err != nil {
		return fmt.Errorf("failed to compute archive hash: %w", err)
	}

	var signature []byte

	// Sign based on key type
	switch s.keyType {
	case KeyTypeRSA:
		signature, err = s.signWithRSA(archiveHash)
		if err != nil {
			return fmt.Errorf("RSA signing failed: %w", err)
		}

	case KeyTypeED25519:
		signature, err = s.signWithED25519(archiveHash)
		if err != nil {
			return fmt.Errorf("Ed25519 signing failed: %w", err)
		}

	default:
		return fmt.Errorf("unknown key type")
	}

	// Write signature to file
	signaturePath := pkg.SignaturePath
	if err := os.WriteFile(signaturePath, signature, 0644); err != nil {
		return fmt.Errorf("failed to write signature: %w", err)
	}

	return nil
}

// signWithRSA signs a hash using RSA-PSS
func (s *Signer) signWithRSA(hash []byte) ([]byte, error) {
	return rsa.SignPSS(
		rand.Reader,
		s.rsaKey,
		crypto.SHA256,
		hash,
		nil,
	)
}

// signWithED25519 signs a hash using Ed25519
func (s *Signer) signWithED25519(hash []byte) ([]byte, error) {
	// Ed25519 signs the message directly
	// We're signing the hash of the archive
	return ed25519.Sign(s.ed25519Key, hash), nil
}

// GetPublicKey returns the public key in SSH authorized_keys format
//
// This public key should be added to the remote system's authorized_keys
// so the agent can verify playbook signatures.
//
// The format is compatible with SSH and can be directly appended to
// ~/.ssh/authorized_keys
func (s *Signer) GetPublicKey() (string, error) {
	var sshPublicKey ssh.PublicKey

	switch s.keyType {
	case KeyTypeRSA:
		sshPublicKey, _ = ssh.NewPublicKey(&s.rsaKey.PublicKey)
	case KeyTypeED25519:
		sshPublicKey, _ = ssh.NewPublicKey(s.ed25519Key.Public())
	default:
		return "", fmt.Errorf("unknown key type")
	}

	// Marshal to SSH authorized_keys format
	authorizedKey := ssh.MarshalAuthorizedKey(sshPublicKey)

	return string(authorizedKey), nil
}

// GetKeyType returns the type of key loaded
func (s *Signer) GetKeyType() KeyType {
	return s.keyType
}

// GetKeyTypeName returns a human-readable name for the key type
func (s *Signer) GetKeyTypeName() string {
	switch s.keyType {
	case KeyTypeRSA:
		return "RSA"
	case KeyTypeED25519:
		return "Ed25519"
	default:
		return "Unknown"
	}
}

// computeFileHash computes the SHA256 hash of a file
func computeFileHash(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(data)
	return hash[:], nil
}

// // GenerateKeyPair generates a new RSA key pair for signing playbooks
// //
// // This is a utility function for initial setup. The private key should be
// // kept secure on the controller, and the public key should be added to
// // the remote system's authorized_keys.
// //
// // Note: For Ed25519 keys, use ssh-keygen instead:
// //
// //	ssh-keygen -t ed25519 -f nestor_controller_key
// func GenerateKeyPair(outputDir string) (privateKeyPath, publicKeyPath string, err error) {
// 	// Generate private key
// 	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
// 	if err != nil {
// 		return "", "", fmt.Errorf("failed to generate private key: %w", err)
// 	}

// 	// Write private key
// 	privateKeyPath = filepath.Join(outputDir, "nestor_controller_key")
// 	privateKeyFile, err := os.OpenFile(privateKeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
// 	if err != nil {
// 		return "", "", fmt.Errorf("failed to create private key file: %w", err)
// 	}
// 	defer privateKeyFile.Close()

// 	privateKeyPEM := &pem.Block{
// 		Type:  "RSA PRIVATE KEY",
// 		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
// 	}

// 	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
// 		return "", "", fmt.Errorf("failed to write private key: %w", err)
// 	}

// 	// Write public key in SSH format
// 	publicKeyPath = filepath.Join(outputDir, "nestor_controller_key.pub")
// 	sshPublicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
// 	if err != nil {
// 		return "", "", fmt.Errorf("failed to create SSH public key: %w", err)
// 	}

// 	authorizedKey := ssh.MarshalAuthorizedKey(sshPublicKey)
// 	if err := os.WriteFile(publicKeyPath, authorizedKey, 0644); err != nil {
// 		return "", "", fmt.Errorf("failed to write public key: %w", err)
// 	}

// 	return privateKeyPath, publicKeyPath, nil
// }
