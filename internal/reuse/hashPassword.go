package reuse

import "golang.org/x/crypto/bcrypt"

// HashPassword hashes a plain password string using Bcrypt (cost 12)
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

// CheckPasswordHash verifies a plain password against an existing bcrypt hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}