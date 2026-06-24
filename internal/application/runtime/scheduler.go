package runtime

import (
	"context"
	"log"
	"strings"
	"time"

	"lightmonitor/internal/application/core"
)

type Scheduler struct {
	service *core.Service
	store   core.Store
}

func NewScheduler(service *core.Service, store core.Store) *Scheduler {
	return &Scheduler{service: service, store: store}
}

func (s *Scheduler) Start(ctx context.Context) {
	go s.loop(ctx, 5*time.Second, s.pollActiveRequests)
	go s.loop(ctx, 15*time.Second, s.checkMissing)
	go s.loop(ctx, time.Hour, s.cleanup)
}

func (s *Scheduler) loop(ctx context.Context, interval time.Duration, fn func(context.Context) error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if err := fn(ctx); err != nil && !isDatabaseNotInstalled(err) {
		log.Printf("scheduler: %v", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := fn(ctx); err != nil && !isDatabaseNotInstalled(err) {
				log.Printf("scheduler: %v", err)
			}
		}
	}
}

func isDatabaseNotInstalled(err error) bool {
	return strings.Contains(err.Error(), "no such table")
}

func (s *Scheduler) pollActiveRequests(ctx context.Context) error {
	requests, err := s.store.ListActiveRequests(ctx, 0)
	if err != nil {
		return err
	}
	for _, req := range requests {
		if !req.Enabled {
			continue
		}
		if err := s.service.PollActiveRequest(ctx, req); err != nil {
			log.Printf("active request %d failed: %v", req.ID, err)
		}
	}
	return nil
}

func (s *Scheduler) checkMissing(ctx context.Context) error {
	return s.service.CheckMissing(ctx)
}

func (s *Scheduler) cleanup(ctx context.Context) error {
	return s.service.Cleanup(ctx)
}
