package vpncore

import (
	"bytes"
	"testing"

	"golang.org/x/crypto/chacha20poly1305"
)

func TestCryptoRoundTrip(t *testing.T) {
	a, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	sa, err := ComputeSharedSecret(a.PrivateKey, b.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	sb, err := ComputeSharedSecret(b.PrivateKey, a.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sa, sb) {
		t.Fatal("shared secrets differ")
	}
	ca, err := NewCipher(sa)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := NewCipher(sb)
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("hello entangled")
	ct, err := ca.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	out, err := cb.Decrypt(ct)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, plain) {
		t.Fatalf("got %q want %q", out, plain)
	}
}

func TestDeriveSessionKeyDeterministic(t *testing.T) {
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i)
	}
	k1, err := DeriveSessionKey(secret)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := DeriveSessionKey(secret)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatal("HKDF not deterministic")
	}
	if bytes.Equal(k1, secret) {
		t.Fatal("HKDF key must differ from raw shared secret")
	}
}

func TestHKDFRejectsRawSecretCiphertext(t *testing.T) {
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 1)
	}
	hk, err := NewCipher(secret)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := chacha20poly1305.NewX(secret)
	if err != nil {
		t.Fatal(err)
	}
	var nonce [chacha20poly1305.NonceSizeX]byte
	ct := raw.Seal(nonce[:], nonce[:], []byte("legacy-payload"), nil)
	if _, err := hk.Decrypt(ct); err == nil {
		t.Fatal("HKDF cipher should reject raw-secret ciphertext")
	}
}

func TestPublicKeyCodec(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	s := EncodePublicKey(kp.PublicKey)
	decoded, err := DecodePublicKey(s)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != kp.PublicKey {
		t.Fatal("roundtrip mismatch")
	}
}

func TestCryptoHKDFConstant(t *testing.T) {
	if CryptoHKDF != "hkdf-v1" {
		t.Fatalf("CryptoHKDF = %q", CryptoHKDF)
	}
}
