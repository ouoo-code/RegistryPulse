package auth

import (
	"testing"
	"time"
)

func TestTOTPCodeRoundTrip(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	secret := "JBSWY3DPEHPK3PXP"
	code := TOTPCode(secret, now)
	if len(code) != 6 || !VerifyTOTP(secret, code, now) {
		t.Fatalf("invalid generated code %q", code)
	}
	if VerifyTOTP(secret, "000000", now) && code != "000000" {
		t.Fatal("invalid code accepted")
	}
}
