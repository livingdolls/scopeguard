package application

import "context"

type HealthResult struct {
	Status string `json:"status"`
}

type CheckHealth interface {
	Execute(ctx context.Context) HealthResult
}

type HealthService struct{}

func NewHealthService() *HealthService {
	return &HealthService{}
}

func (s *HealthService) Execute(ctx context.Context) HealthResult {
	return HealthResult{
		Status: "ok",
	}
}
