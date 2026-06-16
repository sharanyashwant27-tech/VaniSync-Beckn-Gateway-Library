package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// SimpleKeyManager holds an Ed25519 key pair for Beckn message signing.
type SimpleKeyManager struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

// NewSimpleKeyManager generates a fresh Ed25519 key pair.
func NewSimpleKeyManager() (*SimpleKeyManager, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 key: %w", err)
	}
	return &SimpleKeyManager{privateKey: priv, publicKey: pub}, nil
}

// NewSimpleKeyManagerFromSeed loads keys from a 32-byte seed.
func NewSimpleKeyManagerFromSeed(seed []byte) (*SimpleKeyManager, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("seed must be %d bytes", ed25519.SeedSize)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	return &SimpleKeyManager{privateKey: priv, publicKey: pub}, nil
}

// PublicKeyBase64 returns the public key encoded for gateway headers.
func (km *SimpleKeyManager) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(km.publicKey)
}

// Sign returns a base64 Ed25519 signature over payload bytes.
func (km *SimpleKeyManager) Sign(payload []byte) (string, error) {
	sig := ed25519.Sign(km.privateKey, payload)
	return base64.StdEncoding.EncodeToString(sig), nil
}

// Verify checks a base64 signature against payload bytes.
func (km *SimpleKeyManager) Verify(payload []byte, signatureB64 string) (bool, error) {
	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return false, fmt.Errorf("decode signature: %w", err)
	}
	return ed25519.Verify(km.publicKey, payload, sig), nil
}
