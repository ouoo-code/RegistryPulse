package auth

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ouoo-code/RegistryPulse/internal/credential"
)

func encryptTOTPSecret(secret string) ([]byte, []byte, error) {
	key, err := credential.KeyFromEnvironment()
	if err != nil {
		return nil, nil, err
	}
	return credential.Encrypt(secret, key)
}

func decryptTOTPSecret(ciphertext, nonce []byte) (string, error) {
	key, err := credential.KeyFromEnvironment()
	if err != nil {
		return "", err
	}
	return credential.Decrypt(ciphertext, nonce, key)
}

// MigrateTOTPSecrets encrypts legacy plaintext values once after the schema
// migration. It fails closed when a legacy secret exists but no valid key is
// configured, so plaintext is never silently left behind.
func MigrateTOTPSecrets(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database is required")
	}
	rows, err := db.QueryContext(ctx, `SELECT id,totp_secret FROM users WHERE COALESCE(totp_secret,'') <> '' AND COALESCE(totp_secret_ciphertext,''::bytea) = ''::bytea`)
	if err != nil {
		return fmt.Errorf("query legacy TOTP secrets: %w", err)
	}
	defer rows.Close()
	type legacySecret struct {
		id     string
		secret string
	}
	var legacy []legacySecret
	for rows.Next() {
		var item legacySecret
		if err := rows.Scan(&item.id, &item.secret); err != nil {
			return fmt.Errorf("scan legacy TOTP secret: %w", err)
		}
		legacy = append(legacy, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read legacy TOTP secrets: %w", err)
	}
	if len(legacy) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin TOTP migration: %w", err)
	}
	defer tx.Rollback()
	for _, item := range legacy {
		ciphertext, nonce, err := encryptTOTPSecret(item.secret)
		if err != nil {
			return fmt.Errorf("encrypt TOTP secret: %w", err)
		}
		if _, err = tx.ExecContext(ctx, `UPDATE users SET totp_secret_ciphertext=$1,totp_secret_nonce=$2,totp_secret='' WHERE id=$3`, ciphertext, nonce, item.id); err != nil {
			return fmt.Errorf("store encrypted TOTP secret: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit TOTP migration: %w", err)
	}
	return nil
}

func EncryptTOTPSecret(secret string) ([]byte, []byte, error) {
	return encryptTOTPSecret(secret)
}

func DecryptTOTPSecret(ciphertext, nonce []byte) (string, error) {
	return decryptTOTPSecret(ciphertext, nonce)
}
