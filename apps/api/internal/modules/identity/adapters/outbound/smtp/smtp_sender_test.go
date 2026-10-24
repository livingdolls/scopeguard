package smtpadapter

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"mime/quotedprintable"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func validConfig() Config {
	return Config{
		Host:            "smtp.example.com",
		Port:            587,
		TLSMode:         "starttls",
		Username:        "test-user",
		Password:        "test-password",
		From:            "no-reply@example.com",
		VerificationURL: "https://app.example.com/verify-email",
	}
}

func TestNewSender(t *testing.T) {
	sender, err := NewSender(validConfig())
	if err != nil {
		t.Fatalf("create SMTP sender: %v", err)
	}

	if sender == nil {
		t.Fatal("expected sender")
	}
}

func TestNewSenderRejectsInsecureConfig(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Config)
	}{
		{
			name: "plaintext SMTP",
			change: func(c *Config) {
				c.TLSMode = "none"
			},
		},
		{
			name: "HTTP verification URL",
			change: func(c *Config) {
				c.VerificationURL =
					"http://app.example.com/verify-email"
			},
		},
		{
			name: "missing credentials",
			change: func(c *Config) {
				c.Password = ""
			},
		},
		{
			name: "invalid SMTP port",
			change: func(c *Config) {
				c.Port = 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.change(&cfg)

			_, err := NewSender(cfg)
			if err == nil {
				t.Fatal("expected invalid configuration error")
			}
		})
	}
}

func TestBuildVerificationEmail(t *testing.T) {
	deliveryID := uuid.New()

	link := "https://app.example.com/verify-email?token=test"

	message, err := buildMessage(
		"no-reply@example.com",
		"user@example.com",
		link,
		deliveryID,
	)
	if err != nil {
		t.Fatal(err)
	}

	parts := bytes.SplitN(
		message,
		[]byte("\r\n\r\n"),
		2,
	)

	if len(parts) != 2 {
		t.Fatal("missing email headers or body")
	}

	headers := string(parts[0])

	if !strings.Contains(headers, "Message-ID:") ||
		!strings.Contains(headers, deliveryID.String()) {
		t.Fatal("missing stable delivery identifier")
	}

	if !strings.Contains(
		headers,
		"Content-Transfer-Encoding: quoted-printable",
	) {
		t.Fatal("missing body encoding")
	}

	decoded, err := io.ReadAll(
		quotedprintable.NewReader(
			bytes.NewReader(parts[1]),
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(decoded), link) {
		t.Fatal("verification link missing from email body")
	}
}

func TestSMTPSenderCanceledContext(t *testing.T) {
	sender, err := NewSender(validConfig())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	token := base64.RawURLEncoding.EncodeToString(
		bytes.Repeat([]byte{1}, 32),
	)

	err = sender.SendVerification(
		ctx,
		"user@example.com",
		token,
		uuid.New(),
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}
