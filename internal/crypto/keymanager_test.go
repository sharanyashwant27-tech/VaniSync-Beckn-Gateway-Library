package crypto_test

import (
	"testing"

	"github.com/yashwant/vanisync-beckn/internal/crypto"
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
