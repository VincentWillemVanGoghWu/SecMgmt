package service

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

type DeviceCheckScheduler struct {
	platformService *PlatformService
	logger          *zap.Logger
	mu              sync.Mutex
	running         bool
}

func NewDeviceCheckScheduler(platformService *PlatformService, logger *zap.Logger) *DeviceCheckScheduler {
	return &DeviceCheckScheduler{platformService: platformService, logger: logger}
}

func (s *DeviceCheckScheduler) Start() func() {
	if s == nil || s.platformService == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		s.tick(time.Now())
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				s.tick(now)
			}
		}
	}()
	return cancel
}

func (s *DeviceCheckScheduler) tick(now time.Time) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	if s.logger != nil {
		s.logger.Debug("scan device check schedules")
	}
	s.platformService.runDueDeviceCheckSchedules(now)
}
