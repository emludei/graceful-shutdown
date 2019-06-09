package graceful_shutdown

import (
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

type loggerStub struct {
	messages []string
}

func (ls *loggerStub) Log(level LogLevel, msg string) {
	ls.messages = append(ls.messages, msg)
}

func (ls *loggerStub) ContainsError(msg string) bool {
	for _, err := range ls.messages {
		if strings.Contains(err, msg) {
			return true
		}
	}

	return false
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name             string
		shutdownerConfig Config
		wantErr          bool
		errString        string
	}{
		{
			name: "Case-1: valid config, log level none",
			shutdownerConfig: Config{
				ShutdownTimeout: 15 * time.Second,
				Logger:          &loggerStub{},
				LogLevel:        LogLevelNone,
			},
			wantErr: false,
		},
		{
			name: "Case-2: valid config, log level error",
			shutdownerConfig: Config{
				ShutdownTimeout: 15 * time.Second,
				Logger:          &loggerStub{},
				LogLevel:        LogLevelError,
			},
			wantErr: false,
		},
		{
			name: "Case-3: valid config, log level info",
			shutdownerConfig: Config{
				ShutdownTimeout: 15 * time.Second,
				Logger:          &loggerStub{},
				LogLevel:        LogLevelInfo,
			},
			wantErr: false,
		},
		{
			name: "Case-4: valid config, log level debug",
			shutdownerConfig: Config{
				ShutdownTimeout: 15 * time.Second,
				Logger:          &loggerStub{},
				LogLevel:        LogLevelDebug,
			},
			wantErr: false,
		},
		{
			name: "Case-5: invalid shutdown timeout - zero",
			shutdownerConfig: Config{
				ShutdownTimeout: 0,
				Logger:          &loggerStub{},
				LogLevel:        LogLevelDebug,
			},
			wantErr:   true,
			errString: "field ShutdownTimeout <= zero",
		},
		{
			name: "Case-6: invalid shutdown timeout - negative number",
			shutdownerConfig: Config{
				ShutdownTimeout: -100,
				Logger:          &loggerStub{},
				LogLevel:        LogLevelDebug,
			},
			wantErr:   true,
			errString: "field ShutdownTimeout <= zero",
		},
		{
			name: "Case-7: invalid logger - nil",
			shutdownerConfig: Config{
				ShutdownTimeout: 10 * time.Second,
				Logger:          nil,
				LogLevel:        LogLevelNone,
			},
			wantErr:   true,
			errString: "field Logger is nil",
		},
		{
			name: "Case-8: invalid logLevel - lower than was expected",
			shutdownerConfig: Config{
				ShutdownTimeout: 10 * time.Second,
				Logger:          &loggerStub{},
				LogLevel:        0,
			},
			wantErr:   true,
			errString: "field LogLevel has unsupported value",
		},
		{
			name: "Case-9: invalid logLevel - higher than was expected",
			shutdownerConfig: Config{
				ShutdownTimeout: 10 * time.Second,
				Logger:          &loggerStub{},
				LogLevel:        LogLevelDebug + 1,
			},
			wantErr:   true,
			errString: "field LogLevel has unsupported value",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.shutdownerConfig.Validate()

			if err != nil && !test.wantErr {
				t.Errorf("config.Validate() got unexpected error: %v", err)
			}

			if err == nil && test.wantErr {
				t.Errorf("config.Validate() was expecting error (%v) but got nil", test.errString)
			}

			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errString) {
				errString := "config.Validate() (%v) expected error %v, got error %v"
				t.Errorf(errString, test.shutdownerConfig, test.errString, err)
			}
		})
	}
}

type serviceStub struct {
	isStarted bool
	timeout   time.Duration
}

func (s *serviceStub) Stop() error {
	if s.timeout.Nanoseconds() > 0 {
		time.Sleep(s.timeout)
	}

	s.isStarted = false
	return nil
}

func TestShutdownManager_HandleOsSignal_WithoutErrors(t *testing.T) {
	test := struct {
		config    Config
		service   IStoppable
		errString string
	}{
		config: Config{
			LogLevel:        LogLevelError,
			Logger:          &loggerStub{},
			ShutdownTimeout: 2 * time.Second,
		},
		service: &serviceStub{isStarted: true},
	}

	shutdownManager := NewShutdownManager(test.config)
	shutdownManager.AppendServices(test.service)

	err := shutdownManager.RegisterSignals(syscall.SIGTERM)
	if err != nil {
		t.Errorf("shutdownManager.RegisterSignals(syscall.SIGTERM) error: %v", err)
	}

	go func() {
		err := syscall.Kill(os.Getpid(), syscall.SIGTERM)
		if err != nil {
			t.Errorf("syscall.Kill(os.Getpid(), syscall.SIGTERM) error: %v", err)
		}
	}()

	exitCode := shutdownManager.Wait()
	if exitCode != ExitCodeSuccess {
		t.Errorf("shutdownManager.Wait() returned not success exit code %v", exitCode)
	}

	service, ok := test.service.(*serviceStub)
	if !ok {
		t.Errorf("test.service.(*serviceStub) is not ok, type of service is %T", test.service)
	}

	if service.isStarted {
		t.Errorf("service %T is still started", service)
	}

	logger, ok := test.config.Logger.(*loggerStub)
	if !ok {
		errString := "test.config.Logger.(*loggerStub) is not ok, type of logger is %T"
		t.Errorf(errString, test.config.Logger)
	}

	if len(logger.messages) != 0 {
		t.Errorf("got errors from logger:\n%v", strings.Join(logger.messages, "\n"))
	}
}

func TestShutdownManager_HandleOsSignal_ByTimeout(t *testing.T) {
	test := struct {
		config    Config
		service   IStoppable
		errString string
	}{
		config: Config{
			LogLevel:        LogLevelError,
			Logger:          &loggerStub{},
			ShutdownTimeout: 2 * time.Second,
		},
		service:   &serviceStub{isStarted: true, timeout: 3 * time.Second},
		errString: "ShutdownManager.stop() shutdown timeout",
	}

	shutdownManager := NewShutdownManager(test.config)
	shutdownManager.AppendServices(test.service)

	err := shutdownManager.RegisterSignals(syscall.SIGTERM)
	if err != nil {
		t.Errorf("shutdownManager.RegisterSignals(syscall.SIGTERM) error: %v", err)
	}

	go func() {
		err := syscall.Kill(os.Getpid(), syscall.SIGTERM)
		if err != nil {
			t.Errorf("syscall.Kill(os.Getpid(), syscall.SIGTERM) error: %v", err)
		}
	}()

	exitCode := shutdownManager.Wait()
	if exitCode != ExitCodeSuccess {
		t.Errorf("shutdownManager.Wait() returned not success exit code %v", exitCode)
	}

	logger, ok := test.config.Logger.(*loggerStub)
	if !ok {
		errString := "test.config.Logger.(*loggerStub) is not ok, type of logger is %T"
		t.Errorf(errString, test.config.Logger)
	}

	if len(logger.messages) <= 0 {
		t.Errorf("there are no any errors, but there must be some error")
	}

	if !logger.ContainsError(test.errString) {
		errString := "logger does not contain error: '%s' but there are such errors:\n%v"
		t.Errorf(errString, test.errString, strings.Join(logger.messages, "\n"))
	}
}

func TestShutdownManager_ResetSignals(t *testing.T) {
	test := struct {
		config    Config
		service   IStoppable
		errString string
	}{
		config: Config{
			LogLevel:        LogLevelError,
			Logger:          &loggerStub{},
			ShutdownTimeout: 2 * time.Second,
		},
		service: &serviceStub{isStarted: true},
	}

	shutdownManager := NewShutdownManager(test.config)
	shutdownManager.AppendServices(test.service)

	err := shutdownManager.RegisterSignals(syscall.SIGUSR1)
	if err != nil {
		t.Errorf("shutdownManager.RegisterSignals(syscall.SIGUSR1) error: %v", err)
	}

	shutdownManager.ResetSignals(syscall.SIGUSR1)

	go func() {
		err := syscall.Kill(os.Getpid(), syscall.SIGUSR1)
		if err != nil {
			t.Errorf("syscall.Kill(os.Getpid(), syscall.SIGUSR1) error: %v", err)
		}
	}()

	waitDone := make(chan struct{})
	go func() {
		shutdownManager.Wait()
		waitDone <- struct{}{}
	}()

	select {
	case <-waitDone:
		t.Errorf("shutdown by signal syscall.SIGUSR1 that was reseted")
	case <-time.After(3 * time.Second):
	}

	service, ok := test.service.(*serviceStub)
	if !ok {
		t.Errorf("test.service.(*serviceStub) is not ok, type of service is %T", test.service)
	}

	if !service.isStarted {
		t.Errorf("service %T is not started", service)
	}

	logger, ok := test.config.Logger.(*loggerStub)
	if !ok {
		errString := "test.config.Logger.(*loggerStub) is not ok, type of logger is %T"
		t.Errorf(errString, test.config.Logger)
	}

	if len(logger.messages) != 0 {
		t.Errorf("got errors from logger:\n%v", strings.Join(logger.messages, "\n"))
	}
}

func TestShutdownManager_TriggerShutdown(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		service        IStoppable
		wantedExitCode int
		shutdownFunc   func(*ShutdownManager)
	}{
		{
			name: "Case-1: trigger general shutdown",
			config: Config{
				LogLevel:        LogLevelError,
				Logger:          &loggerStub{},
				ShutdownTimeout: 2 * time.Second,
			},
			service:        &serviceStub{isStarted: true},
			wantedExitCode: ExitCodeSuccess,
			shutdownFunc: func(manager *ShutdownManager) {
				time.Sleep(1 * time.Second)
				manager.TriggerShutdown()
			},
		},
		{
			name: "Case-2: trigger fatal shutdown",
			config: Config{
				LogLevel:        LogLevelError,
				Logger:          &loggerStub{},
				ShutdownTimeout: 2 * time.Second,
			},
			service:        &serviceStub{isStarted: true},
			wantedExitCode: ExitCodeError,
			shutdownFunc: func(manager *ShutdownManager) {
				time.Sleep(1 * time.Second)
				manager.TriggerFatal()
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shutdownManager := NewShutdownManager(test.config)
			shutdownManager.AppendServices(test.service)

			go test.shutdownFunc(shutdownManager)

			exitCode := shutdownManager.Wait()
			if exitCode != test.wantedExitCode {
				t.Errorf("shutdownManager.Wait() returned not success exit code %v", exitCode)
			}

			service, ok := test.service.(*serviceStub)
			if !ok {
				t.Errorf("test.service.(*serviceStub) is not ok, type of service is %T", test.service)
			}

			if service.isStarted {
				t.Errorf("service %T is still started", service)
			}

			logger, ok := test.config.Logger.(*loggerStub)
			if !ok {
				errString := "test.config.Logger.(*loggerStub) is not ok, type of logger is %T"
				t.Errorf(errString, test.config.Logger)
			}

			if len(logger.messages) != 0 {
				t.Errorf("got errors from logger:\n%v", strings.Join(logger.messages, "\n"))
			}
		})
	}
}
