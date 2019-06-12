# graceful-shutdown [![Build Status](https://travis-ci.org/emludei/graceful-shutdown.svg?branch=master)](https://travis-ci.org/emludei/graceful-shutdown)


This is a Go package that provides a convenient way to control of shutdown your services inside your program.

## Installing

To install, simply execute:

```bash
go get github.com/emludei/graceful-shutdown
```

## Usage

At first, you need to create a shutdown manager via `NewShutdownManager` function that requires of config object `gs.Config` with parameters:

* *ShutdownTimeout*: a timeout for stopping services, if services can't stop in specified timeout, the shutdown manager will log error message.
* *Logger*: adapter for your logger that implements gs.ILogger interface.
* *LogLevel*: level of logging.

```go
package main

import (
    ...
    gs "github.com/emludei/graceful-shutdown"
    ...
)

func main() {
    cfg := gs.Config{
        ShutdownTimeout: 5 * time.Second,
        Logger:          logAdapter, // log adapter that have to implement gs.ILogger interface
        LogLevel:        gs.LogLevelInfo,
    }
    if err := cfg.Validate(); err != nil {
        // Handle error
    }

    shutdownManager := gs.NewShutdownManager(cfg)
}

```

If you want to trigger graceful shutdown inside your services, for example in case of some events or errors/panics - you can inject shutdown manager to your services (using `gs.IShutdownManager` interface) and use `TriggerShutdown` method for normal shutdown (`Wait` method will return success exit code 0) or `TriggerFatal` method for fatal shutdown (`Wait` method will return error exit code 1).

```go
    ...
    someServiceA := NewSomeServiceA(shutdownManager)
    someServiceB := NewSomeServiceB(shutdownManager)
    someServiceC := NewSomeServiceC(shutdownManager)

    someServiceA.Start()
    someServiceB.Start()
    someServiceC.Start()
    ...
```

To tell shutdown manager what services to control, you should use `AppendServices` method. But services that are passed to this method must implement `gs.IStoppable` interface (they have to implement `Stop() error` method) and they have to know how to stop themselves correctly via this method.

```go
    ...
    shutdownManager.AppendServices(someServiceA, someServiceB, someServiceC)
    ...
```

Also you can add handling of OS signals via `RegisterSignals` method and cancel handling of OS signals via `ResetSignals` method.

```go
    ...
    err = shutdownManager.RegisterSignals(syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
    if err != nil {
        // Handle error
    }

    shutdownManager.ResetSignals(syscall.SIGQUIT)
    ...
```

After all, you need to call `Wait` method that will block execution until shutdown will be triggered (if stopping services exceeds `ShutdownTimeout` timeout, the error message will be logged and the `Wait` will return).

```go
    ...
    os.Exit(shutdownManager.Wait())
    ...
```

For more details see [example](https://github.com/emludei/graceful-shutdown/blob/master/examples/simple/simple.go).