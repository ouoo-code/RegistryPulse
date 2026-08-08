package auth

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// VerifyTOTP validates a six-digit RFC 6238 code in the small +/- one-step
// clock-skew window normally used by authenticator applications.
func VerifyTOTP(secret, code string, now time.Time) bool {
	secret = strings.ToUpper(strings.TrimSpace(secret))
	secret = strings.TrimRight(secret, "=")
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil || len(key) == 0 || len(strings.TrimSpace(code)) != 6 {
		return false
	}
	for offset := int64(-1); offset <= 1; offset++ {
		counter := uint64(now.Unix()/30 + offset)
		message := []byte{byte(counter >> 56), byte(counter >> 48), byte(counter >> 40), byte(counter >> 32), byte(counter >> 24), byte(counter >> 16), byte(counter >> 8), byte(counter)}
		mac := hmac.New(sha1.New, key)
		_, _ = mac.Write(message)
		sum := mac.Sum(nil)
		index := sum[len(sum)-1] & 0x0f
		value := (uint32(sum[index])&0x7f)<<24 | uint32(sum[index+1])<<16 | uint32(sum[index+2])<<8 | uint32(sum[index+3])
		if hmac.Equal([]byte(fmt.Sprintf("%06d", value%1000000)), []byte(strings.TrimSpace(code))) {
			return true
		}
	}
	return false
}

func TOTPCode(secret string, now time.Time) string {
	// Useful for deterministic tests and provisioning tools.
	secret = strings.TrimRight(strings.ToUpper(strings.TrimSpace(secret)), "=")
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	counter := uint64(now.Unix() / 30)
	message := []byte{byte(counter >> 56), byte(counter >> 48), byte(counter >> 40), byte(counter >> 32), byte(counter >> 24), byte(counter >> 16), byte(counter >> 8), byte(counter)}
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(message)
	sum := mac.Sum(nil)
	index := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[index])&0x7f)<<24 | uint32(sum[index+1])<<16 | uint32(sum[index+2])<<8 | uint32(sum[index+3])
	return fmt.Sprintf("%06s", strconv.FormatUint(uint64(value%1000000), 10))
}
