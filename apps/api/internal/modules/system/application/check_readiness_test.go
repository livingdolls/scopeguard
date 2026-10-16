package application

import (
	"context"
	"errors"
	"testing"
)

type fakeDatabaseProbe struct {
	err error
}

func (f *fakeDatabaseProbe) Ping(
	ctx context.Context,
) error {
	return f.err
}

func TestReadinessServiceSuccess(t *testing.T) {
	probe := &fakeDatabaseProbe{}

	service := NewReadinessService(probe)

	err := service.Execute(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadinessServiceFailure(t *testing.T) {
	dbErr := errors.New("connection refused")

	probe := &fakeDatabaseProbe{
		err: dbErr,
	}

	service := NewReadinessService(probe)

	err := service.Execute(context.Background())

	if !errors.Is(err, dbErr) {
		t.Fatalf(
			"expected database error, got %v",
			err,
		)
	}
}
