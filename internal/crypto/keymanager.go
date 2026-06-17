package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SimpleKeyManager holds an Ed25519 key pair for Beckn message signing.
type SimpleKeyManager struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

type keyFile struct {
	Seed string `json:"seed"`
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

// LoadSimpleKeyManager loads an Ed25519 key pair from a JSON key file.
func LoadSimpleKeyManager(path string) (*SimpleKeyManager, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}
	return loadSimpleKeyManagerFromBytes(data)
}

// LoadOrCreateSimpleKeyManager loads a key from path or generates and persists a new one.
func LoadOrCreateSimpleKeyManager(path string) (*SimpleKeyManager, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			km, err := NewSimpleKeyManager()
			if err != nil {
				return nil, err
			}
			if err := km.Save(path); err != nil {
				return nil, err
			}
			return km, nil
		}
		return nil, fmt.Errorf("read key file: %w", err)
	}
	return loadSimpleKeyManagerFromBytes(data)
}

func loadSimpleKeyManagerFromBytes(data []byte) (*SimpleKeyManager, error) {
	var kf keyFile
	if err := json.Unmarshal(data, &kf); err != nil {
		return nil, fmt.Errorf("parse key file: %w", err)
	}
	seed, err := base64.StdEncoding.DecodeString(kf.Seed)
	if err != nil {
		return nil, fmt.Errorf("decode key seed: %w", err)
	}
	return NewSimpleKeyManagerFromSeed(seed)
}

// Save persists the private key seed to path with restrictive permissions.
func (km *SimpleKeyManager) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create key directory: %w", err)
	}

	seed := km.privateKey.Seed()
	data, err := json.Marshal(keyFile{Seed: base64.StdEncoding.EncodeToString(seed)})
	if err != nil {
		return fmt.Errorf("marshal key file: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write key file: %w", err)
	}
	return nil
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
