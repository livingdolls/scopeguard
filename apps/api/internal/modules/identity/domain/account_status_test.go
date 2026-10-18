package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAccountStatusCanAuthenticate(t *testing.T) {
	cases := []struct {
		status  AccountStatus
		allowed bool
	}{
		{AccountPendingVerification, false},
		{AccountActive, true},
		{AccountSuspended, false},
	}

	for _, tt := range cases {
		if got := tt.status.CanAuthenticate(); got != tt.allowed {
			t.Errorf(
				"status %s: expected %t, got %t",
				tt.status, tt.allowed, got,
			)
		}
	}
}

func TestNewUserStartsPendingVerification(t *testing.T) {
	user, err := NewUser(
		uuid.New(),
		"user@example.com",
		time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if user.Status() != AccountPendingVerification {
		t.Fatalf("unexpected initial status: %s", user.Status())
	}
}
