package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

var ErrEncryptionKey = errors.New("credential encryption key is missing or invalid")

func KeyFromEnvironment() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("CREDENTIAL_ENCRYPTION_KEY"))
	if raw == "" {
		return nil, ErrEncryptionKey
	}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(raw); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if len(raw) == 32 {
		return []byte(raw), nil
	}
	return nil, ErrEncryptionKey
}

func Encrypt(plaintext string, key []byte) (ciphertext, nonce []byte, err error) {
	if len(key) != 32 {
		return nil, nil, ErrEncryptionKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("create credential cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("create credential gcm: %w", err)
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("create credential nonce: %w", err)
	}
	return gcm.Seal(nil, nonce, []byte(plaintext), nil), nonce, nil
}

func Decrypt(ciphertext, nonce, key []byte) (string, error) {
	if len(key) != 32 || len(ciphertext) == 0 || len(nonce) == 0 {
		return "", ErrEncryptionKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create credential cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create credential gcm: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("credential decryption failed")
	}
	return string(plaintext), nil
}

func Fingerprint(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func Last4(secret string) string {
	runes := []rune(secret)
	if len(runes) > 4 {
		runes = runes[len(runes)-4:]
	}
	return string(runes)
}

func Mask(secret string) string {
	if secret == "" {
		return ""
	}
	last4 := Last4(secret)
	return "****" + last4
}
