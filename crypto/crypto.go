package crypto

import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

// GenerateX25519KeyPair generates a new X25519 key pair for ECDH.
// Returns (publicKeyBytes, privateKeyBytes, error).
func GenerateX25519KeyPair() ([]byte, []byte, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return priv.PublicKey().Bytes(), priv.Bytes(), nil
}

// DeriveSharedSecret computes the shared secret using ECDH.
func DeriveSharedSecret(privateKeyBytes []byte, publicKeyBytes []byte) ([]byte, error) {
	priv, err := ecdh.X25519().NewPrivateKey(privateKeyBytes)
	if err != nil {
		return nil, err
	}
	pub, err := ecdh.X25519().NewPublicKey(publicKeyBytes)
	if err != nil {
		return nil, err
	}
	return priv.ECDH(pub)
}

// DeriveSessionKey derives a 32-byte key from a shared secret using HKDF-SHA256.
func DeriveSessionKey(sharedSecret []byte, salt []byte, info []byte) ([]byte, error) {
	hkdfReader := hkdf.New(sha256.New, sharedSecret, salt, info)
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// Encrypt encrypts standard plaintext using a symmetric 32-byte key with ChaCha20-Poly1305.
// The output contains the random nonce prefixed to the ciphertext.
func Encrypt(key []byte, plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Seal appends the ciphertext to the nonce, return value starts with nonce
	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext using a symmetric 32-byte key with ChaCha20-Poly1305.
func Decrypt(key []byte, ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}

	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, encryptedPayload := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, encryptedPayload, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
