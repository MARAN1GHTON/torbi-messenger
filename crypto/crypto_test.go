package crypto

import (
	"bytes"
	"testing"
)

func TestCryptoFlow(t *testing.T) {
	// 1. Generate key pairs for A and B
	pubA, privA, err := GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("GenerateX25519KeyPair for A failed: %v", err)
	}
	pubB, privB, err := GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("GenerateX25519KeyPair for B failed: %v", err)
	}

	// 2. Perform ECDH from A's perspective and B's perspective
	secretA, err := DeriveSharedSecret(privA, pubB)
	if err != nil {
		t.Fatalf("DeriveSharedSecret A failed: %v", err)
	}
	secretB, err := DeriveSharedSecret(privB, pubA)
	if err != nil {
		t.Fatalf("DeriveSharedSecret B failed: %v", err)
	}

	if !bytes.Equal(secretA, secretB) {
		t.Fatalf("Shared secrets mismatch: A derived %x, B derived %x", secretA, secretB)
	}

	// 3. Derive session keys via HKDF
	keyA, err := DeriveSessionKey(secretA, nil, []byte("test-chat"))
	if err != nil {
		t.Fatalf("DeriveSessionKey A failed: %v", err)
	}
	keyB, err := DeriveSessionKey(secretB, nil, []byte("test-chat"))
	if err != nil {
		t.Fatalf("DeriveSessionKey B failed: %v", err)
	}

	if !bytes.Equal(keyA, keyB) {
		t.Fatalf("Derived session keys mismatch")
	}

	// 4. Encrypt and Decrypt
	plaintext := []byte("Decentralized P2P message")
	ciphertext, err := Encrypt(keyA, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := Decrypt(keyB, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("Decrypted plaintext mismatch: expected %q, got %q", string(plaintext), string(decrypted))
	}
}
