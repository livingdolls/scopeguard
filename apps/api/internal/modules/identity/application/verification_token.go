package application

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"
)

const verificationTokenTTL = 24 * time.Hour
const verificationTokenBytes = 32

type generatedVerificationToken struct {
	Raw       string
	Hash      []byte
	ExpiresAt time.Time
}

func generateVerificationToken(now time.Time) (generatedVerificationToken, error) {
	raw := make([]byte, verificationTokenBytes)

	if _, err := rand.Read(raw); err != nil {
		return generatedVerificationToken{}, fmt.Errorf("generate verification token: %w", err)
	}

	digest := sha256.Sum256(raw)

	return generatedVerificationToken{
		Raw:       base64.RawURLEncoding.EncodeToString(raw),
		Hash:      digest[:],
		ExpiresAt: now.UTC().Add(verificationTokenTTL),
	}, nil
}
