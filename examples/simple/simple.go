package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	gs "github.com/emludei/graceful-shutdown"
	"github.com/emludei/graceful-shutdown/log/zapadapter"
	"go.uber.org/zap"
)

type Service struct {
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup

	shutdownManager gs.IShutdownManager
}

func (s *Service) Start() {
	s.ctx, s.cancelFunc = context.WithCancel(context.Background())

	s.wg.Add(1)
	go s.worker()
}

// The service should know how to stop itself correctly
func (s *Service) Stop() error {
	if s.cancelFunc == nil {
		return fmt.Errorf("Service.Stop() cancelFunc is nil")
	}

	s.cancelFunc()
	s.wg.Wait()

	return nil
}

func (s *Service) worker() {
	defer func() {
		if r := recover(); r != nil {
			// Log error...

			// We will shutdown program with fatal signal if some service (in our program) unexpectedly goes down
			s.shutdownManager.TriggerFatal()
		}

		s.wg.Done()
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(10 * time.Second):
			fmt.Println("hello from worker!")
		}
	}
}

func NewService(shutdownManager gs.IShutdownManager) *Service {
	service := &Service{
		wg:              sync.WaitGroup{},
		shutdownManager: shutdownManager,
	}

	return service
}

var (
	exitCode = gs.ExitCodeSuccess
)

func main() {
	defer func(code *int) { os.Exit(*code) }(&exitCode)

	defer func() {
		if r := recover(); r != nil {
			// Handle error...

			exitCode = gs.ExitCodeError
		}
	}()

	zapLogger, err := zap.NewProduction()
	if err != nil {
		// Handle error
	}
	defer func() {
		err := zapLogger.Sync()
		if err != nil {
			// Handle error
		}
	}()

	logAdapter := zapadapter.NewLogger(zapLogger)

	cfg := gs.Config{
		ShutdownTimeout: 5 * time.Second,
		Logger:          logAdapter,
		LogLevel:        gs.LogLevelInfo,
	}
	if err := cfg.Validate(); err != nil {
		// Handle error
	}

	shutdownManager := gs.NewShutdownManager(cfg)

	firstService := NewService(shutdownManager)
	secondService := NewService(shutdownManager)
	thirdService := NewService(shutdownManager)

	firstService.Start()
	secondService.Start()
	thirdService.Start()

	shutdownManager.AppendServices(firstService, secondService, thirdService)

	err = shutdownManager.RegisterSignals(syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	if err != nil {
		// Handle error
	}

	exitCode = shutdownManager.Wait()
}
