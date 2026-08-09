package domain

import "testing"

func TestProbeModeAndCategoryDefaultsMatchConfigurationInheritance(t *testing.T) {
	store := NewMemoryStore()
	custom := true
	source, err := store.UpsertSource(SourceInput{
		CategoryID: "ghcr", Name: "custom mode", BaseURL: "https://ghcr.example", ProbeConfigCustom: custom, ProbeMode: "http",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := store.Source(source.ID); got.ProbeMode != "http" {
		t.Fatalf("custom probe mode = %q, want http", got.ProbeMode)
	}

	custom = false
	if _, err := store.UpsertSource(SourceInput{CategoryID: "ghcr", Name: "category mode", BaseURL: "https://ghcr.example", ProbeConfigCustom: custom, ProbeMode: "http"}, source.ID); err != nil {
		t.Fatal(err)
	}
	got, err := store.Source(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProbeMode != "manifest" {
		t.Fatalf("category probe mode = %q, want manifest", got.ProbeMode)
	}
}
