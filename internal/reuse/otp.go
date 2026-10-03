package reuse

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
)

// GenerateCryptoSecureOTP generates an unpredictable 6-digit numeric OTP (100000 - 999999)
func GenerateCryptoSecureOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", fmt.Errorf("failed to generate secure OTP: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()+100000), nil
}

// HashOTP computes a SHA-256 keyed hash combining user identifier, OTP, and secret
func HashOTP(identifier, otp, secret string) string {
	h := sha256.New()
	h.Write([]byte(identifier + ":" + otp + ":" + secret))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyOTPHash validates an input OTP against an expected hash in constant time
func VerifyOTPHash(identifier, otp, secret, expectedHash string) bool {
	computed := HashOTP(identifier, otp, secret)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(expectedHash)) == 1
}