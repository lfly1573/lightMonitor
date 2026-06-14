package monitor

import "context"

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) ReceivePassiveSample(ctx context.Context) error {
	_ = ctx
	return nil
}
