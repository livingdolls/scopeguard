package encryption

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestAESGCMSealAndOpen(t *testing.T) {
	sealer, err := NewAESGCM(
		[]byte(strings.Repeat("k", 32)),
		"v1",
	)
	if err != nil {
		t.Fatal(err)
	}

	eventID := uuid.New()

	first, err := sealer.Seal(
		eventID,
		"user@example.com",
		"verification-secret",
	)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Contains(
		first.Data,
		[]byte("verification-secret"),
	) {
		t.Fatal("encrypted payload contains plaintext token")
	}

	message, err := sealer.Open(eventID, first)
	if err != nil {
		t.Fatal(err)
	}

	if message.To != "user@example.com" ||
		message.Token != "verification-secret" {
		t.Fatal("decrypted message differs from original")
	}

	// Every encryption should have a different random nonce.
	second, err := sealer.Seal(
		eventID,
		"user@example.com",
		"verification-secret",
	)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(first.Data, second.Data) {
		t.Fatal("different encryptions produced identical data")
	}

	// Ciphertext must not be interchangeable between events.
	if _, err := sealer.Open(uuid.New(), first); err == nil {
		t.Fatal("wrong event ID must fail")
	}

	// Tampering must be detected.
	first.Data[len(first.Data)-1] ^= 0xff

	if _, err := sealer.Open(eventID, first); err == nil {
		t.Fatal("tampered ciphertext must fail")
	}
}
