package encryption

import (
	"bytes"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	key1, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() returned error: %v", err)
	}
	if len(key1) != KeySize {
		t.Fatalf("expected key of length %d, got %d", KeySize, len(key1))
	}

	key2, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() returned error: %v", err)
	}
	if bytes.Equal(key1, key2) {
		t.Fatal("expected two generated keys to be different, got identical keys")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() returned error: %v", err)
	}

	original := []byte("some secret odometer reading data")

	ciphertext, nonce, err := Encrypt(original, key)
	if err != nil {
		t.Fatalf("Encrypt() returned error: %v", err)
	}

	if bytes.Equal(ciphertext, original) {
		t.Fatal("ciphertext should not equal plaintext")
	}
	if len(nonce) == 0 {
		t.Fatal("expected non-empty nonce")
	}

	decrypted, err := Decrypt(ciphertext, nonce, key)
	if err != nil {
		t.Fatalf("Decrypt() returned error: %v", err)
	}

	if !bytes.Equal(decrypted, original) {
		t.Fatalf("decrypted data does not match original: got %q, want %q", decrypted, original)
	}
}

func TestEncrypt_DifferentNoncePerCall(t *testing.T) {
	key, _ := GenerateKey()
	plaintext := []byte("same data")

	_, nonce1, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt() returned error: %v", err)
	}
	_, nonce2, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt() returned error: %v", err)
	}

	// Same plaintext + same key should still produce a different nonce
	// each time, since it's freshly randomly generated per call.
	if bytes.Equal(nonce1, nonce2) {
		t.Fatal("expected different nonce for repeated encryption calls")
	}
}

func TestDecrypt_WrongKeyFails(t *testing.T) {
	key, _ := GenerateKey()
	wrongKey, _ := GenerateKey()

	ciphertext, nonce, err := Encrypt([]byte("secret data"), key)
	if err != nil {
		t.Fatalf("Encrypt() returned error: %v", err)
	}

	_, err = Decrypt(ciphertext, nonce, wrongKey)
	if err == nil {
		t.Fatal("expected Decrypt() with wrong key to fail, got nil error")
	}
}

func TestDecrypt_CorruptedCiphertextFails(t *testing.T) {
	key, _ := GenerateKey()
	ciphertext, nonce, err := Encrypt([]byte("secret data"), key)
	if err != nil {
		t.Fatalf("Encrypt() returned error: %v", err)
	}

	corrupted := append([]byte{}, ciphertext...)
	corrupted[len(corrupted)/2] ^= 0xFF

	_, err = Decrypt(corrupted, nonce, key)
	if err == nil {
		t.Fatal("expected Decrypt() with corrupted ciphertext to fail, got nil error")
	}
}

func TestDecrypt_WrongNonceFails(t *testing.T) {
	key, _ := GenerateKey()
	ciphertext, _, err := Encrypt([]byte("secret data"), key)
	if err != nil {
		t.Fatalf("Encrypt() returned error: %v", err)
	}

	_, wrongNonce, _ := Encrypt([]byte("other"), key) // valid nonce, but wrong one

	_, err = Decrypt(ciphertext, wrongNonce, key)
	if err == nil {
		t.Fatal("expected Decrypt() with wrong nonce to fail, got nil error")
	}
}

func TestEncrypt_InvalidKeySize(t *testing.T) {
	_, _, err := Encrypt([]byte("data"), []byte("too-short-key"))
	if err == nil {
		t.Fatal("expected Encrypt() with invalid key size to fail, got nil error")
	}
}
