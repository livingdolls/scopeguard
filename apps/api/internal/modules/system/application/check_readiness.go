package application

import (
	"context"
	"fmt"
)

// DatabaseProbe is an outbound port
type DatabaseProbe interface {
	Ping(ctx context.Context) error
}

// CheckReadiness is an inbound port
type CheckReadiness interface {
	Execute(ctx context.Context) error
}

type ReadinessService struct {
	database DatabaseProbe
}

func NewReadinessService(database DatabaseProbe) *ReadinessService {
	return &ReadinessService{
		database: database,
	}
}

func (s *ReadinessService) Execute(ctx context.Context) error {
	if err := s.database.Ping(ctx); err != nil {
		return fmt.Errorf("database readiness check failed: %w", err)
	}

	return nil
}
