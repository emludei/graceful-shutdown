package graceful_shutdown

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"time"
)

const (
	sigChanBufferSize = 1
)

// Exit codes
const (
	ExitCodeSuccess = 0
	ExitCodeError   = 1
)

// Config contains all the options for shutdown manager.
type Config struct {
	ShutdownTimeout time.Duration
	Logger          ILogger
	LogLevel        LogLevel
}

// Validate validates config options.
func (sc Config) Validate() error {
	if sc.ShutdownTimeout.Nanoseconds() <= 0 {
		return fmt.Errorf("Config.Validate() field ShutdownTimeout <= zero: %v", sc.ShutdownTimeout)
	}

	if sc.Logger == nil {
		return errors.New("Config.Validate() field Logger is nil")
	}

	if sc.LogLevel < LogLevelNone || sc.LogLevel > LogLevelDebug {
		return fmt.Errorf("Config.Validate() field LogLevel has unsupported value: %v", sc.LogLevel)
	}

	return nil
}

// String returns human readable string representation of config options.
func (sc Config) String() string {
	reprString := "Config{ShutdownTimeout: %s, Logger: %s, LogLevel: %s}"
	return fmt.Sprintf(reprString, sc.ShutdownTimeout, sc.Logger, sc.LogLevel)
}

// ShutdownManager shutdowns stoppable services by signal.
type ShutdownManager struct {
	shutdownTimeout time.Duration
	logger          ILogger
	logLevel        LogLevel

	services    []IStoppable
	signalsChan chan os.Signal
}

// AppendServices appends stoppable services for shutdown by receiving signal.
func (s *ShutdownManager) AppendServices(services ...IStoppable) {
	s.services = append(s.services, services...)
}

// RegisterSignals registers OS signals for shutdown.
func (s *ShutdownManager) RegisterSignals(signals ...os.Signal) (err error) {
	defer func() {
		if r := recover(); r != nil {
			msg := "ShutdownManager.RegisterSignals(%v) panic: (%v), at: [[\n%s\n]]\n"
			err = fmt.Errorf(msg, signals, r, debug.Stack())
		}
	}()

	// It can rise panic
	signal.Notify(s.signalsChan, signals...)

	return err
}

// ResetSignals undoes the effect of any prior calls to RegisterSignals for the provided signals.
func (s *ShutdownManager) ResetSignals(signals ...os.Signal) {
	signal.Reset(signals...)
}

// TriggerShutdown trigger inner Shutdown signal (In this case the Wait method will return success exit code - 0).
func (s *ShutdownManager) TriggerShutdown() {
	if s.logLevel >= LogLevelInfo {
		s.logger.Log(LogLevelInfo, "ShutdownManager.TriggerShutdown() trigger shutdown signal")
	}

	select {
	case s.signalsChan <- SignalShutdown:
	default:
	}
}

// TriggerFatal trigger inner Fatal signal (In this case the Wait method will return error exit code - 1).
func (s *ShutdownManager) TriggerFatal() {
	if s.logLevel >= LogLevelInfo {
		s.logger.Log(LogLevelInfo, "ShutdownManager.TriggerFatal trigger fatal signal")
	}

	select {
	case s.signalsChan <- SignalFatal:
	default:
	}
}

// Wait blocks until get signal (os or inner), then stop all tracked services.
func (s *ShutdownManager) Wait() int {
	sig, ok := <-s.signalsChan
	if !ok && s.logLevel >= LogLevelError {
		s.logger.Log(LogLevelError, "ShutdownManager.Wait() channel with os signals was unexpectedly closed")
	}

	if s.logLevel >= LogLevelInfo {
		msg := fmt.Sprintf("ShutdownManager.Wait() got signal %s", sig)
		s.logger.Log(LogLevelInfo, msg)
	}

	deadline := time.Now().UTC().Add(s.shutdownTimeout)
	ctx, cancelFunc := context.WithDeadline(context.Background(), deadline)

	s.stop(ctx)

	cancelFunc()

	switch sig {
	case SignalFatal:
		if s.logLevel >= LogLevelInfo {
			msg := fmt.Sprintf("ShutdownManager.Wait() fatal exit with code %v", ExitCodeError)
			s.logger.Log(LogLevelInfo, msg)
		}

		return ExitCodeError
	default:
		if s.logLevel >= LogLevelInfo {
			msg := fmt.Sprintf("ShutdownManager.Wait() exit with code %v", ExitCodeSuccess)
			s.logger.Log(LogLevelInfo, msg)
		}

		return ExitCodeSuccess
	}
}

func (s *ShutdownManager) stop(ctx context.Context) {
	done := make(chan struct{})

	go func() {
		for _, service := range s.services {
			err := service.Stop()
			if err != nil && s.logLevel >= LogLevelError {
				msg := fmt.Sprintf("ShutdownManager.stop() stop service %T error: %v", service, err)
				s.logger.Log(LogLevelError, msg)
			}
		}

		done <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		if s.logLevel >= LogLevelError {
			msg := fmt.Sprintf("ShutdownManager.stop() shutdown timeout (%s) was exceeded", s.shutdownTimeout)
			s.logger.Log(LogLevelError, msg)
		}
	case <-done:
		if s.logLevel >= LogLevelInfo {
			s.logger.Log(LogLevelInfo, "ShutdownManager.stop() all services were successfully stopped")
		}
	}
}

// NewShutdownManager creates and returns new instance of ShutdownManager, using provided config options.
func NewShutdownManager(cfg Config) *ShutdownManager {
	shutdowner := &ShutdownManager{
		shutdownTimeout: cfg.ShutdownTimeout,
		logger:          cfg.Logger,
		logLevel:        cfg.LogLevel,
		services:        make([]IStoppable, 0),
		signalsChan:     make(chan os.Signal, sigChanBufferSize),
	}

	return shutdowner
}
