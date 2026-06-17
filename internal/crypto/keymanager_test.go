package crypto_test

import (
	"path/filepath"
	"sync"
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

func TestLoadOrCreateConcurrentUsesSameKey(t *testing.T) {
	t.Parallel()

	keyPath := filepath.Join(t.TempDir(), "ed25519.key")
	const workers = 16

	var wg sync.WaitGroup
	pubs := make([]string, workers)
	errs := make([]error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			km, err := crypto.LoadOrCreateSimpleKeyManager(keyPath)
			errs[i] = err
			if km != nil {
				pubs[i] = km.PublicKeyBase64()
			}
		}(i)
	}
	wg.Wait()

	var ref string
	for i := 0; i < workers; i++ {
		if errs[i] != nil {
			t.Fatalf("worker %d: %v", i, errs[i])
		}
		if pubs[i] == "" {
			t.Fatalf("worker %d: empty public key", i)
		}
		if ref == "" {
			ref = pubs[i]
		} else if pubs[i] != ref {
			t.Fatalf("worker %d public key mismatch", i)
		}
	}
}
