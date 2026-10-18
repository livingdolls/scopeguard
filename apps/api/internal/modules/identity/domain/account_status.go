package domain

type AccountStatus string

const (
	AccountPendingVerification AccountStatus = "pending_verification"
	AccountActive              AccountStatus = "active"
	AccountSuspended           AccountStatus = "suspended"
)

func (s AccountStatus) CanAuthenticate() bool {
	return s == AccountActive
}
