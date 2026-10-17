package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{
			name:  "valid passphrase",
			value: "correct horse battery staple",
		},
		{
			name:    "too short",
			value:   "short123",
			wantErr: true,
		},
		{
			name:    "empty password",
			value:   "",
			wantErr: true,
		},
		{
			name:    "too long",
			value:   strings.Repeat("a", 1025),
			wantErr: true,
		},
		{
			name:    "invalid UTF-8",
			value:   string([]byte{0xff, 0xfe}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.value)

			if tt.wantErr {
				if !errors.Is(err, ErrInvalidPassword) {
					t.Fatalf(
						"expected invalid password, got %v",
						err,
					)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
