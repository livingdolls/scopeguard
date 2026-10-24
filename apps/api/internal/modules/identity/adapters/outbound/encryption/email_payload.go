package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
)

type VerificationEmail = application.VerificationEmail

type AESGCM struct {
	aead  cipher.AEAD
	keyID string
}

var _ application.EmailPayloadSealer = (*AESGCM)(nil)
var _ application.EmailPayloadOpener = (*AESGCM)(nil)

func NewAESGCM(key []byte, keyID string) (*AESGCM, error) {
	if len(key) != 32 {
		return nil, errors.New("email encryption key must be 32 bytes long")
	}

	if keyID == "" {
		return nil, errors.New("email encryption key is empty")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher block: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM cipher: %w", err)
	}

	return &AESGCM{aead: aead, keyID: keyID}, nil

}

func associateData(eventID uuid.UUID) []byte {
	return []byte("scopeguard:identity.email_verification" + eventID.String())
}

func (s *AESGCM) Seal(eventID uuid.UUID, email string, token string) (application.EncryptedEmailPayload, error) {
	if eventID == uuid.Nil || email == "" || token == "" {
		return application.EncryptedEmailPayload{}, errors.New("invalid verification email data")
	}

	plaintext, err := json.Marshal(VerificationEmail{
		To:    email,
		Token: token,
	})

	if err != nil {
		return application.EncryptedEmailPayload{}, err
	}

	// random nonce must be used to each encryption
	nonce := make([]byte, s.aead.NonceSize())

	if _, err := rand.Read(nonce); err != nil {
		return application.EncryptedEmailPayload{}, err
	}

	ciphertext := s.aead.Seal(nil, nonce, plaintext, associateData(eventID))

	//persist nonce followed by ciphertext and authentication tag
	encrypted := make([]byte, 0, len(nonce)+len(ciphertext))
	encrypted = append(encrypted, nonce...)
	encrypted = append(encrypted, ciphertext...)

	return application.EncryptedEmailPayload{
		KeyID: s.keyID,
		Data:  encrypted,
	}, nil

}

func (s *AESGCM) Open(eventID uuid.UUID, payload application.EncryptedEmailPayload) (VerificationEmail, error) {
	if eventID == uuid.Nil || payload.KeyID != s.keyID {
		return VerificationEmail{}, errors.New("invalid email encryption metadata")
	}

	nonceSize := s.aead.NonceSize()

	if len(payload.Data) < nonceSize+s.aead.Overhead() {
		return VerificationEmail{}, errors.New("invalid email encryption payload")
	}

	nonce := payload.Data[:nonceSize]
	ciphertext := payload.Data[nonceSize:]

	plaintext, err := s.aead.Open(nil, nonce, ciphertext, associateData(eventID))
	if err != nil {
		return VerificationEmail{}, errors.New("email payload authentication failed")
	}

	var message VerificationEmail

	if err := json.Unmarshal(plaintext, &message); err != nil {
		return VerificationEmail{}, err
	}

	if message.To == "" || message.Token == "" {
		return VerificationEmail{}, errors.New("invalid decryption email message")
	}

	return message, nil
}
