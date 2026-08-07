package vpncore

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

type KeyPair struct {
	PrivateKey [32]byte
	PublicKey  [32]byte
}

func EncodePublicKey(pub [32]byte) string {
	return base64.StdEncoding.EncodeToString(pub[:])
}

func DecodePublicKey(s string) ([32]byte, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return [32]byte{}, err
	}
	if len(b) != 32 {
		return [32]byte{}, errors.New("invalid public key length")
	}
	var key [32]byte
	copy(key[:], b)
	return key, nil
}

func GenerateKeyPair() (*KeyPair, error) {
	var priv [32]byte
	if _, err := io.ReadFull(rand.Reader, priv[:]); err != nil {
		return nil, err
	}
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return nil, err
	}
	var pubKey [32]byte
	copy(pubKey[:], pub)
	return &KeyPair{PrivateKey: priv, PublicKey: pubKey}, nil
}

func ComputeSharedSecret(private, public [32]byte) ([]byte, error) {
	return curve25519.X25519(private[:], public[:])
}

// CryptoHKDF is the only supported session-key mode (advertised in peer_info).
const CryptoHKDF = "hkdf-v1"

// DeriveSessionKey runs HKDF-SHA256 over the raw X25519 shared secret.
func DeriveSessionKey(sharedSecret []byte) ([]byte, error) {
	r := hkdf.New(sha256.New, sharedSecret, nil, []byte("entangled-vpn-v1"))
	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, err
	}
	return key, nil
}

type Cipher struct {
	aead cipher.AEAD
}

// NewCipher derives an HKDF session key and builds an XChaCha20-Poly1305 AEAD.
func NewCipher(sharedSecret []byte) (*Cipher, error) {
	key, err := DeriveSessionKey(sharedSecret)
	if err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	var nonce [chacha20poly1305.NonceSizeX]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce[:], nonce[:], plaintext, nil), nil
}

func (c *Cipher) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < chacha20poly1305.NonceSizeX {
		return nil, errors.New("ciphertext too short")
	}
	return c.aead.Open(nil, ciphertext[:chacha20poly1305.NonceSizeX], ciphertext[chacha20poly1305.NonceSizeX:], nil)
}
