package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/livingdolls/scopeguard/apps/api/internal/platform/config"
)

type workerConfig struct {
	DatabaseURL      string
	DBMaxConnections int32

	EncryptionKey   []byte
	EncryptionKeyID string

	SMTPHost     string
	SMTPPort     int
	SMTPTLSMode  string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	VerificationURL string
	Concurrency     int
}

func requiredEnv(name string) (string, error) {
	value := os.Getenv(name)

	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s must be set", name)
	}

	return value, nil
}

func loadWorkerConfig() (workerConfig, error) {
	dbConfig, err := config.Load()
	if err != nil {
		return workerConfig{}, err
	}

	host, err := requiredEnv("SMTP_HOST")
	if err != nil {
		return workerConfig{}, err
	}

	username, err := requiredEnv("SMTP_USERNAME")
	if err != nil {
		return workerConfig{}, err
	}

	password, err := requiredEnv("SMTP_PASSWORD")
	if err != nil {
		return workerConfig{}, err
	}

	from, err := requiredEnv("SMTP_FROM")
	if err != nil {
		return workerConfig{}, err
	}

	verificationURL, err := requiredEnv("EMAIL_VERIFICATION_URL")
	if err != nil {
		return workerConfig{}, err
	}

	keyB64, err := requiredEnv("EMAIL_OUTBOX_KEY_B64")
	if err != nil {
		return workerConfig{}, err
	}

	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil || len(key) != 32 {
		return workerConfig{}, errors.New("EMAIL_OUTBOX_KEY_B64 not base64")
	}

	keyID, err := requiredEnv("EMAIL_OUTBOX_KEY_ID")
	if err != nil {
		return workerConfig{}, err
	}

	port, err := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if err != nil || port < 1 || port > 65535 {
		return workerConfig{}, errors.New("SMTP_PORT must be an integer")
	}

	mode, err := requiredEnv("SMTP_TLS_MODE")
	if err != nil {
		return workerConfig{}, err
	}

	concurrency := 2

	if value := os.Getenv("EMAIL_WORKER_CONCURRENCY"); value != "" {
		concurrency, err = strconv.Atoi(value)
		if err != nil || concurrency < 1 || concurrency > 20 {
			return workerConfig{}, errors.New("SMTP_CONCURRENCY must be an 1-20")
		}
	}

	return workerConfig{
		DatabaseURL:      dbConfig.DatabaseURL,
		DBMaxConnections: dbConfig.DBMaxConnections,

		EncryptionKey:   key,
		EncryptionKeyID: keyID,
		SMTPHost:        host,
		SMTPPort:        port,
		SMTPTLSMode:     mode,
		SMTPUsername:    username,
		SMTPPassword:    password,
		SMTPFrom:        from,
		VerificationURL: verificationURL,
		Concurrency:     concurrency,
	}, nil

}
