package application

type PasswordHasher interface {
	Hash(plainPassword string) (string, error)
	Verify(plainPassword string, encodeHash string) (bool, error)
}
