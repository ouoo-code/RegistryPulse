package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
)

// These tests use a caller-provided PostgreSQL database because the selection
// contract is implemented by SQL joins and foreign keys, not by the memory
// store. They create and remove uniquely named records and never use a
// production database unless the caller explicitly supplies its DSN.
func openTestImageDatabase(t *testing.T) (*Store, *testImageFixture) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_IMAGE_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("set TEST_IMAGE_TEST_DATABASE_URL (or DATABASE_URL) to run test-image PostgreSQL integration tests")
	}
	t.Setenv("DATABASE_URL", dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	db, err := Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Initialize(ctx, db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatal(err)
	}

	suffix := time.Now().UTC().Format("20060102150405.000000000")
	fixture := &testImageFixture{categoryID: "test-images-" + suffix}
	if err := db.QueryRowContext(ctx, `
		INSERT INTO registry_categories(id,slug,name,description,default_test_repository,default_test_tag,default_probe_mode,default_timeout_seconds,auth_type)
		VALUES($1,$1,$1,'integration test','test/repository','category-tag','manifest',22,'none') RETURNING id`, fixture.categoryID).Scan(&fixture.categoryID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM registry_sources WHERE category_id=$1`, fixture.categoryID)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM registry_categories WHERE id=$1`, fixture.categoryID)
		for _, id := range fixture.imageIDs {
			_, _ = db.ExecContext(cleanupCtx, `DELETE FROM test_images WHERE id=$1`, id)
		}
	})
	return NewStore(db), fixture
}

type testImageFixture struct {
	categoryID string
	imageIDs   []string
}

func (f *testImageFixture) image(t *testing.T, db *Store, reference string, enabled, isDefault bool) string {
	t.Helper()
	var id string
	if err := db.DB.QueryRow(`INSERT INTO test_images(reference,enabled,max_bytes,is_default) VALUES($1,$2,1234,$3) RETURNING id::text`, reference, enabled, isDefault).Scan(&id); err != nil {
		t.Fatal(err)
	}
	f.imageIDs = append(f.imageIDs, id)
	return id
}

func TestTestImageSelectionPriorityAndUnrestrictedFallback(t *testing.T) {
	store, fixture := openTestImageDatabase(t)
	var globalID, globalReference string
	if err := store.DB.QueryRow(`SELECT id::text,reference FROM test_images WHERE is_default AND enabled ORDER BY id LIMIT 1`).Scan(&globalID, &globalReference); err != nil {
		t.Fatalf("find seeded global default test image: %v", err)
	}
	categoryID := fixture.image(t, store, "test/category:latest", true, false)
	sourceID := fixture.image(t, store, "test/source:latest", true, false)

	if _, err := store.DB.Exec(`UPDATE registry_categories SET default_test_image_id=$1 WHERE id=$2`, categoryID, fixture.categoryID); err != nil {
		t.Fatal(err)
	}
	custom := true
	mode := "http"
	imageID := sourceID
	source, err := store.UpsertSource(domain.SourceInput{
		CategoryID: fixture.categoryID, Name: "selection source", BaseURL: "https://registry.example",
		ProbeConfigCustom: custom, ProbeMode: mode, TestImageID: &imageID,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if source.TestImageID != sourceID || source.TestImageReference != "test/source:latest" || source.ProbeMode != "http" {
		t.Fatalf("source-specific selection = %+v", source)
	}

	custom = false
	if _, err := store.UpsertSource(domain.SourceInput{CategoryID: fixture.categoryID, Name: "selection source", BaseURL: "https://registry.example", ProbeConfigCustom: custom, ProbeMode: mode}, source.ID); err != nil {
		t.Fatal(err)
	}
	categorySource, err := store.Source(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if categorySource.TestImageID != categoryID || categorySource.TestImageReference != "test/category:latest" || categorySource.ProbeMode != "manifest" {
		t.Fatalf("category-default selection = %+v", categorySource)
	}

	if _, err := store.DB.Exec(`UPDATE test_images SET enabled=false WHERE id=$1`, categoryID); err != nil {
		t.Fatal(err)
	}
	unrestricted, err := store.Source(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unrestricted.TestImageID != globalID || unrestricted.TestImageReference != globalReference {
		t.Fatalf("unrestricted global fallback = %+v", unrestricted)
	}
}

func TestInvalidTestImageIDIsRejectedByForeignKey(t *testing.T) {
	store, fixture := openTestImageDatabase(t)
	missingID := "00000000-0000-0000-0000-000000000001"
	_, err := store.UpsertSource(domain.SourceInput{
		CategoryID: fixture.categoryID, Name: "invalid image source", BaseURL: "https://registry.example", TestImageID: &missingID,
	}, "")
	if err == nil {
		t.Fatal("expected a missing test_image_id to be rejected")
	}
	if !strings.Contains(strings.ToLower(fmt.Sprint(err)), "foreign key") && !strings.Contains(strings.ToLower(fmt.Sprint(err)), "test_image") {
		t.Fatalf("unexpected invalid test_image_id error: %v", err)
	}
}
