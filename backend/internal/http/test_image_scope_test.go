package httpapi

import "testing"

func TestValidateTestImageModes(t *testing.T) {
	got, err := validateTestImageModes([]string{"manifest", "registry", "manifest"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "manifest" || got[1] != "registry" {
		t.Fatalf("modes = %#v", got)
	}
	if _, err := validateTestImageModes([]string{"unsupported"}); err == nil {
		t.Fatal("unsupported mode must fail")
	}
}

func TestTestImageReferenceValidation(t *testing.T) {
	valid := []string{"library/alpine:latest", "ghcr.io/acme/tool:v1.2.3"}
	for _, reference := range valid {
		if !testImageReference.MatchString(reference) {
			t.Fatalf("expected valid reference %q", reference)
		}
	}
	for _, reference := range []string{"library/alpine", "../secret:latest", "library/alpine:", "Library/alpine:latest"} {
		if testImageReference.MatchString(reference) {
			t.Fatalf("expected invalid reference %q", reference)
		}
	}
}
