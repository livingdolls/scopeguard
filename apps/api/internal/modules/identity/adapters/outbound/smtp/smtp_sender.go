package smtpadapter

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
)

type Config struct {
	Host            string
	Port            int
	TLSMode         string
	Username        string
	Password        string
	From            string
	VerificationURL string
}

type Sender struct {
	cfg        Config
	verifyPage *url.URL
}

var _ application.EmailSender = (*Sender)(nil)

func NewSender(cfg Config) (*Sender, error) {
	if cfg.Host == "" || cfg.Port < 1 || cfg.Port > 65535 {
		return nil, errors.New("invalid SMTP host or port")
	}

	if cfg.TLSMode != "starttls" && cfg.TLSMode != "tls" {
		return nil, errors.New("SMTP TLS mode must be starttls or tls")
	}

	if cfg.Username == "" || cfg.Password == "" {
		return nil, errors.New("SMTP credentials are required")
	}

	from, err := mail.ParseAddress(cfg.From)
	if err != nil || from.Address != cfg.From || from.Name != "" {
		return nil, errors.New("invalid SMTP sender address")
	}

	verifyPage, err := url.Parse(cfg.VerificationURL)
	if err != nil ||
		verifyPage.Scheme != "https" ||
		verifyPage.Host == "" ||
		verifyPage.User != nil ||
		verifyPage.Fragment != "" ||
		verifyPage.RawQuery != "" {
		return nil, errors.New("verification URL must be an HTTPS page URL")
	}

	return &Sender{
		cfg:        cfg,
		verifyPage: verifyPage,
	}, nil
}

func (s *Sender) SendVerification(ctx context.Context, to string, token string, deliveryID uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if deliveryID == uuid.Nil {
		return errors.New("invalid delivery id")
	}

	recipient, err := mail.ParseAddress(to)
	if err != nil || recipient.Address != to || recipient.Name != "" {
		return errors.New("invalid recipient address")
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != token {
		return errors.New("invalid verification token format")
	}

	verifyURL := *s.verifyPage
	query := verifyURL.Query()
	query.Set("token", token)
	verifyURL.RawQuery = query.Encode()

	message, err := buildMessage(s.cfg.From, to, verifyURL.String(), deliveryID)
	if err != nil {
		return err
	}

	address := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(30 * time.Second)

	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}

	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set SMTP deadline failed: %w", err)
	}

	// close the socket when context cancel
	stop := context.AfterFunc(ctx, func() {
		_ = conn.Close()
	})
	defer stop()

	tlsConfig := &tls.Config{
		ServerName: s.cfg.Host,
		MinVersion: tls.VersionTLS12,
	}

	var smtpConn net.Conn = conn

	if s.cfg.TLSMode == "tls" {
		secureConn := tls.Client(conn, tlsConfig)

		if err := secureConn.HandshakeContext(ctx); err != nil {
			return fmt.Errorf("SMTP TLS handshake failed: %w", err)
		}

		smtpConn = secureConn
	}

	client, err := smtp.NewClient(smtpConn, s.cfg.Host)
	if err != nil {
		return fmt.Errorf("failed to initialize SMTP client: %w", err)
	}
	defer client.Close()

	if s.cfg.TLSMode == "starttls" {
		supported, _ := client.Extension("STARTTLS")

		if !supported {
			return errors.New("STARTTLS mode not supported")
		}

		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed to start SMTP STARTTLS: %w", err)
		}
	}

	// authentication is performed only after TLS is established
	supported, _ := client.Extension("AUTH")
	if !supported {
		return errors.New("AUTH mode not supported")
	}

	auth := smtp.PlainAuth(
		"",
		s.cfg.Username,
		s.cfg.Password,
		s.cfg.Host,
	)

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("failed to authenticate SMTP AUTH: %w", err)
	}

	if err := client.Mail(s.cfg.From); err != nil {
		return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("SMTP RCPT TO failed: %w", err)
	}

	writer, err := client.Data()

	if err != nil {
		return fmt.Errorf("SMTP DATA WRITER FAILED: %w", err)
	}

	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP FAILED: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("complete SMTP message: %w", err)
	}

	// A successfully DATA completion means the SMTP server
	// accepted the message, not that it reached inbox
	_ = client.Quit()

	return nil
}

func buildMessage(from string, to string, verificationLink string, deliveryID uuid.UUID) ([]byte, error) {
	if strings.ContainsAny(from+to, "\r\n") {
		return nil, errors.New("invalid email header")
	}

	fromDomain := from[strings.LastIndex(from, "@")+1:]

	if fromDomain == "" || strings.ContainsAny(fromDomain, "<>\r\n @") {
		return nil, errors.New("invalid sender domain")
	}

	body := fmt.Sprintf(
		"Welcome to ScopeGuard!\r\n\r\n"+
			"Please verify your email address by opening:\r\n\r\n"+
			"%s\r\n\r\n"+
			"This link expires in 24 hours.\r\n"+
			"If you did not create an account, ignore this email.\r\n",
		verificationLink,
	)

	var message bytes.Buffer

	headers := fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: Verify your ScopeGuard email\r\n"+
			"Date: %s\r\n"+
			"Message-ID: <scopeguard-%s@%s>\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/plain; charset=UTF-8\r\n"+
			"Content-Transfer-Encoding: quoted-printable\r\n"+
			"\r\n",
		(&mail.Address{Address: from}).String(),
		(&mail.Address{Address: to}).String(),
		time.Now().UTC().Format(time.RFC1123Z),
		deliveryID.String(),
		fromDomain,
	)

	if _, err := message.WriteString(headers); err != nil {
		return nil, err
	}

	encoder := quotedprintable.NewWriter(&message)

	if _, err := encoder.Write([]byte(body)); err != nil {
		return nil, err
	}

	if err := encoder.Close(); err != nil {
		return nil, err
	}

	return message.Bytes(), nil
}
