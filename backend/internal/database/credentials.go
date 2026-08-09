package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/ouoo-code/RegistryPulse/internal/credential"
	"github.com/ouoo-code/RegistryPulse/internal/domain"
)

func QueryCredentialProfiles(ctx context.Context, db *sql.DB) ([]domain.CredentialProfile, error) {
	rows, err := db.QueryContext(ctx, `SELECT id::text,name,auth_type,username,source_id::text,registry_host,category_id,enabled,(secret_ciphertext <> ''::bytea),secret_last4,created_at,updated_at FROM credential_profiles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.CredentialProfile, 0)
	for rows.Next() {
		var item domain.CredentialProfile
		var sourceID, categoryID sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &item.AuthType, &item.Username, &sourceID, &item.RegistryHost, &categoryID, &item.Enabled, &item.HasSecret, &item.SecretMasked, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if sourceID.Valid {
			item.SourceID = sourceID.String
		}
		if categoryID.Valid {
			item.CategoryID = categoryID.String
		}
		if item.HasSecret {
			item.SecretMasked = "****" + item.SecretMasked
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func NormalizeRegistryHost(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if parsed, err := url.Parse(raw); err == nil && parsed.Hostname() != "" {
		raw = strings.ToLower(parsed.Hostname())
	}
	return strings.TrimSuffix(raw, ".")
}

func ValidateCredentialSelectors(ctx context.Context, db *sql.DB, in domain.CredentialProfileInput) error {
	in.SourceID = strings.TrimSpace(in.SourceID)
	in.RegistryHost = NormalizeRegistryHost(in.RegistryHost)
	in.CategoryID = strings.TrimSpace(in.CategoryID)
	if in.SourceID == "" && in.RegistryHost == "" && in.CategoryID == "" {
		return errors.New("at least one credential selector is required")
	}
	if in.SourceID != "" {
		var sourceHost, sourceCategory string
		if err := db.QueryRowContext(ctx, `SELECT registry_host,category_id FROM registry_sources WHERE id=$1`, in.SourceID).Scan(&sourceHost, &sourceCategory); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		if in.RegistryHost != "" && in.RegistryHost != NormalizeRegistryHost(sourceHost) {
			return errors.New("registry_host does not match source_id")
		}
		if in.CategoryID != "" && in.CategoryID != sourceCategory {
			return errors.New("category_id does not match source_id")
		}
	}
	if in.CategoryID != "" {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM registry_categories WHERE id=$1)`, in.CategoryID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return domain.ErrNotFound
		}
	}
	return nil
}

func UpsertCredentialProfile(ctx context.Context, db *sql.DB, in domain.CredentialProfileInput) (domain.CredentialProfile, error) {
	key, err := credential.KeyFromEnvironment()
	if err != nil {
		return domain.CredentialProfile{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	in.AuthType = strings.ToLower(strings.TrimSpace(in.AuthType))
	in.SourceID = strings.TrimSpace(in.SourceID)
	in.RegistryHost = NormalizeRegistryHost(in.RegistryHost)
	in.CategoryID = strings.TrimSpace(in.CategoryID)
	if in.Name == "" || (in.AuthType != "basic" && in.AuthType != "bearer" && in.AuthType != "token") {
		return domain.CredentialProfile{}, errors.New("invalid credential profile")
	}
	if err := ValidateCredentialSelectors(ctx, db, in); err != nil {
		return domain.CredentialProfile{}, err
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	if in.ID == "" && strings.TrimSpace(in.Secret) == "" {
		return domain.CredentialProfile{}, errors.New("secret is required when creating a credential profile")
	}
	var ciphertext, nonce []byte
	var fingerprint, last4 string
	if strings.TrimSpace(in.Secret) != "" {
		ciphertext, nonce, err = credential.Encrypt(in.Secret, key)
		if err != nil {
			return domain.CredentialProfile{}, err
		}
		fingerprint, last4 = credential.Fingerprint(in.Secret), credential.Last4(in.Secret)
	}
	if in.ID == "" {
		err = db.QueryRowContext(ctx, `INSERT INTO credential_profiles(name,auth_type,username,secret_ciphertext,secret_nonce,secret_fingerprint,secret_last4,source_id,registry_host,category_id,enabled) VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::uuid,$9,NULLIF($10,''),$11) RETURNING id::text`, in.Name, in.AuthType, in.Username, ciphertext, nonce, fingerprint, last4, in.SourceID, in.RegistryHost, in.CategoryID, enabled).Scan(&in.ID)
	} else if strings.TrimSpace(in.Secret) != "" || in.ClearSecret {
		if in.ClearSecret && strings.TrimSpace(in.Secret) == "" {
			ciphertext, nonce, fingerprint, last4 = []byte{}, []byte{}, "", ""
		}
		_, err = db.ExecContext(ctx, `UPDATE credential_profiles SET name=$1,auth_type=$2,username=$3,secret_ciphertext=$4,secret_nonce=$5,secret_fingerprint=$6,secret_last4=$7,source_id=NULLIF($8,'')::uuid,registry_host=$9,category_id=NULLIF($10,''),enabled=$11,updated_at=now() WHERE id=$12`, in.Name, in.AuthType, in.Username, ciphertext, nonce, fingerprint, last4, in.SourceID, in.RegistryHost, in.CategoryID, enabled, in.ID)
	} else {
		_, err = db.ExecContext(ctx, `UPDATE credential_profiles SET name=$1,auth_type=$2,username=$3,source_id=NULLIF($4,'')::uuid,registry_host=$5,category_id=NULLIF($6,''),enabled=$7,updated_at=now() WHERE id=$8`, in.Name, in.AuthType, in.Username, in.SourceID, in.RegistryHost, in.CategoryID, enabled, in.ID)
	}
	if err != nil {
		return domain.CredentialProfile{}, err
	}
	return credentialProfileByID(ctx, db, in.ID)
}

func credentialProfileByID(ctx context.Context, db *sql.DB, id string) (domain.CredentialProfile, error) {
	var item domain.CredentialProfile
	var sourceID, categoryID sql.NullString
	var hasSecret bool
	var last4 string
	err := db.QueryRowContext(ctx, `SELECT id::text,name,auth_type,username,source_id::text,registry_host,category_id,enabled,(secret_ciphertext <> ''::bytea),secret_last4,created_at,updated_at FROM credential_profiles WHERE id=$1`, id).Scan(&item.ID, &item.Name, &item.AuthType, &item.Username, &sourceID, &item.RegistryHost, &categoryID, &item.Enabled, &hasSecret, &last4, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return item, domain.ErrNotFound
	}
	if err != nil {
		return item, err
	}
	item.HasSecret = hasSecret
	if hasSecret {
		item.SecretMasked = "****" + last4
	}
	if sourceID.Valid {
		item.SourceID = sourceID.String
	}
	if categoryID.Valid {
		item.CategoryID = categoryID.String
	}
	return item, nil
}

func DeleteCredentialProfile(ctx context.Context, db *sql.DB, id string) error {
	result, err := db.ExecContext(ctx, `DELETE FROM credential_profiles WHERE id=$1`, id)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func ResolveCredential(ctx context.Context, db *sql.DB, sourceID, registryHost, categoryID string) (domain.ResolvedCredential, bool, error) {
	var authType, username string
	var ciphertext, nonce []byte
	err := db.QueryRowContext(ctx, `
		SELECT auth_type,username,secret_ciphertext,secret_nonce
		FROM credential_profiles
		WHERE enabled
		AND (source_id IS NULL OR source_id=NULLIF($1,'')::uuid)
		AND (registry_host='' OR lower(registry_host)=lower($2))
		AND (category_id IS NULL OR category_id=$3)
		ORDER BY (source_id IS NOT NULL) DESC, (registry_host <> '') DESC, (category_id IS NOT NULL) DESC, updated_at DESC
		LIMIT 1`, sourceID, NormalizeRegistryHost(registryHost), categoryID).Scan(&authType, &username, &ciphertext, &nonce)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ResolvedCredential{}, false, nil
	}
	if err != nil {
		return domain.ResolvedCredential{}, false, err
	}
	key, err := credential.KeyFromEnvironment()
	if err != nil {
		return domain.ResolvedCredential{}, false, err
	}
	secret, err := credential.Decrypt(ciphertext, nonce, key)
	if err != nil {
		return domain.ResolvedCredential{}, false, fmt.Errorf("decrypt credential profile: %w", err)
	}
	return domain.ResolvedCredential{AuthType: authType, Username: username, Secret: secret}, true, nil
}
