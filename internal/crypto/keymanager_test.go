package crypto_test

import (
	"path/filepath"
	"testing"

	"github.com/sharanyashwant27-tech/vanisync-beckn/internal/crypto"
)

func TestSignAndVerify(t *testing.T) {
	t.Parallel()

	km, err := crypto.NewSimpleKeyManager()
	if err != nil {
		t.Fatalf("new key manager: %v", err)
	}

	payload := []byte(`{"context":{"action":"confirm"}}`)
	sig, err := km.Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	ok, err := km.Verify(payload, sig)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !ok {
		t.Fatal("signature should verify")
	}
}

func TestLoadOrCreatePersistsKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "ed25519.key")

	km1, err := crypto.LoadOrCreateSimpleKeyManager(keyPath)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	pub1 := km1.PublicKeyBase64()

	km2, err := crypto.LoadSimpleKeyManager(keyPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if km2.PublicKeyBase64() != pub1 {
		t.Fatal("loaded key should match persisted key")
	}
}
