package auth

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost controls the hashing cost. This is intentionally
	// conservative for a local media server; adjust via env if needed.
	BcryptCost = bcrypt.DefaultCost

	// LoginTimingMitigationHash is a bcrypt hash of a fixed passphrase. When no user row exists,
	// login still runs CompareHashAndPassword against this value so timing does not reveal whether
	// the email is registered (see Login / DeviceLogin handlers).
	LoginTimingMitigationHash = `$2a$10$xcZFe5r2fCNWiKJnq.Qog.B4de8owge/5Xfh9OCYX1PLFgc/tVvXS`
)

func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func CheckPasswordHash(plain, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}

func NewSessionID() (string, error) {
	const size = 32
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// SessionLifetime returns the default lifetime for sessions.
func SessionLifetime() time.Duration {
	return 30 * 24 * time.Hour
}
