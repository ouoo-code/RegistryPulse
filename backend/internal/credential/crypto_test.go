package credential

import "testing"

func TestEncryptDecryptAndMask(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	ciphertext, nonce, err := Encrypt("top-secret-token", key)
	if err != nil {
		t.Fatal(err)
	}
	if string(ciphertext) == "top-secret-token" {
		t.Fatal("secret must not be stored as plaintext")
	}
	if got, err := Decrypt(ciphertext, nonce, key); err != nil || got != "top-secret-token" {
		t.Fatalf("decrypt = %q, %v", got, err)
	}
	if got := Mask("top-secret-token"); got != "****oken" {
		t.Fatalf("mask = %q", got)
	}
	if got := Fingerprint("top-secret-token"); got == "" {
		t.Fatal("fingerprint is empty")
	}
}

func TestDecryptRejectsWrongKey(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	otherKey := []byte("abcdefghijklmnopqrstuvwxyz123456")
	ciphertext, nonce, err := Encrypt("secret", key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(ciphertext, nonce, otherKey); err == nil {
		t.Fatal("wrong key must fail")
	}
}
