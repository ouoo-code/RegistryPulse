package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
)

var ErrTestImageNotApplicable = errors.New("test image is not applicable")

func QueryTestImages(ctx context.Context, db *sql.DB, categoryID, probeMode string) ([]domain.TestImage, error) {
	categoryID = strings.TrimSpace(categoryID)
	probeMode = strings.TrimSpace(probeMode)
	rows, err := db.QueryContext(ctx, `
		SELECT id::text,reference,enabled,max_bytes,is_default,auth_strategy,created_at,updated_at
		FROM test_images
		WHERE ($1='' OR NOT EXISTS (
			SELECT 1 FROM test_image_categories tic WHERE tic.test_image_id=test_images.id
		) OR EXISTS (
			SELECT 1 FROM test_image_categories tic WHERE tic.test_image_id=test_images.id AND tic.category_id=$1
		))
		AND ($2='' OR NOT EXISTS (
			SELECT 1 FROM test_image_probe_modes tipm WHERE tipm.test_image_id=test_images.id
		) OR EXISTS (
			SELECT 1 FROM test_image_probe_modes tipm WHERE tipm.test_image_id=test_images.id AND tipm.probe_mode=$2
		))
		ORDER BY reference`, categoryID, probeMode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.TestImage, 0)
	for rows.Next() {
		var item domain.TestImage
		if err := rows.Scan(&item.ID, &item.Reference, &item.Enabled, &item.MaxBytes, &item.IsDefault, &item.AuthStrategy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if err := loadTestImageRelations(ctx, db, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func loadTestImageRelations(ctx context.Context, db *sql.DB, item *domain.TestImage) error {
	categoryRows, err := db.QueryContext(ctx, `SELECT category_id FROM test_image_categories WHERE test_image_id=$1 ORDER BY category_id`, item.ID)
	if err != nil {
		return err
	}
	for categoryRows.Next() {
		var value string
		if err := categoryRows.Scan(&value); err != nil {
			categoryRows.Close()
			return err
		}
		item.CategoryIDs = append(item.CategoryIDs, value)
	}
	if err := categoryRows.Err(); err != nil {
		categoryRows.Close()
		return err
	}
	categoryRows.Close()
	modeRows, err := db.QueryContext(ctx, `SELECT probe_mode FROM test_image_probe_modes WHERE test_image_id=$1 ORDER BY probe_mode`, item.ID)
	if err != nil {
		return err
	}
	for modeRows.Next() {
		var value string
		if err := modeRows.Scan(&value); err != nil {
			modeRows.Close()
			return err
		}
		item.ProbeModes = append(item.ProbeModes, value)
	}
	if err := modeRows.Err(); err != nil {
		modeRows.Close()
		return err
	}
	return modeRows.Close()
}

func TestImageApplicable(ctx context.Context, db *sql.DB, imageID, categoryID, probeMode string) error {
	var applicable bool
	err := db.QueryRowContext(ctx, `
		SELECT enabled
		AND ($2='' OR NOT EXISTS (SELECT 1 FROM test_image_categories WHERE test_image_id=test_images.id)
			OR EXISTS (SELECT 1 FROM test_image_categories WHERE test_image_id=test_images.id AND category_id=$2))
		AND ($3='' OR NOT EXISTS (SELECT 1 FROM test_image_probe_modes WHERE test_image_id=test_images.id)
			OR EXISTS (SELECT 1 FROM test_image_probe_modes WHERE test_image_id=test_images.id AND probe_mode=$3))
		FROM test_images WHERE id=$1`, imageID, strings.TrimSpace(categoryID), strings.TrimSpace(probeMode)).Scan(&applicable)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if !applicable {
		return ErrTestImageNotApplicable
	}
	return nil
}

func ValidateSourceTestImage(ctx context.Context, db *sql.DB, imageID, categoryID, probeMode string) error {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return nil
	}
	categoryID = strings.TrimSpace(categoryID)
	if categoryID == "" {
		return errors.New("category_id is required when test_image_id is set")
	}
	probeMode = strings.TrimSpace(probeMode)
	if probeMode == "" {
		if err := db.QueryRowContext(ctx, `SELECT default_probe_mode FROM registry_categories WHERE id=$1`, categoryID).Scan(&probeMode); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
	}
	return TestImageApplicable(ctx, db, imageID, categoryID, probeMode)
}
