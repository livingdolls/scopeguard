package token

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
)

type accessClaims struct {
	TokenUse string `json:"token_use"`
	jwt.RegisteredClaims
}

type JWTAccessToken struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
	now      func() time.Time
}

var (
	_ application.AccessTokenIssuer   = (*JWTAccessToken)(nil)
	_ application.AccessTokenVerifier = (*JWTAccessToken)(nil)
)

func NewJWTAccessToken(secret []byte, issuer string, audience string, ttl time.Duration) (*JWTAccessToken, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("secret must be at least 32 bytes")
	}

	if strings.TrimSpace(issuer) == "" || strings.TrimSpace(audience) == "" {
		return nil, fmt.Errorf("issuer and audience must not be empty")
	}

	if ttl <= 0 || ttl > time.Hour {
		return nil, fmt.Errorf("ttl must be between 0 and 1 hour")
	}

	return &JWTAccessToken{
		secret:   append([]byte(nil), secret...),
		issuer:   issuer,
		audience: audience,
		ttl:      ttl,
		now:      time.Now,
	}, nil
}

func (s JWTAccessToken) Verify(rawToken string) (uuid.UUID, error) {
	claims := &accessClaims{}

	parsed, err := jwt.ParseWithClaims(
		rawToken,
		claims,
		func(token *jwt.Token) (any, error) {
			return s.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(s.issuer),
		jwt.WithAudience(s.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(5*time.Second),
		jwt.WithTimeFunc(s.now),
	)

	if err != nil || !parsed.Valid {
		return uuid.Nil, application.ErrInvalidAccessToken
	}

	if claims.TokenUse != "access" || claims.IssuedAt == nil || claims.NotBefore == nil || claims.ID == "" {
		return uuid.Nil, application.ErrInvalidAccessToken
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, application.ErrInvalidAccessToken
	}

	return userID, nil
}

func (s JWTAccessToken) Issue(userID uuid.UUID) (string, time.Time, error) {
	if userID == uuid.Nil {
		return "", time.Time{}, fmt.Errorf("userID cannot be nil")
	}

	now := s.now().UTC()
	expiresAt := now.Add(s.ttl)

	claims := accessClaims{
		TokenUse:  "access",
		Issuer:    s.issuer,
		Subject:   userID.String(),
		Audience:  jwt.ClaimStrings{s.audience},
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		ID:        uuid.New().String(),
	}

	signedToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}

	return signedToken, expiresAt, nil
}
