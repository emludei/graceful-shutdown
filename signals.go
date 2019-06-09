package graceful_shutdown

import (
	"strconv"
)

// Signals
const (
	SignalShutdown Signal = 1
	SignalFatal    Signal = 2
)

// Signal table
var signals = [...]string{
	1: "shutdown",
	2: "fatal",
}

// A Signal is a number describing a signal.
// It implements the os.Signal interface.
type Signal int

func (s Signal) Signal() {}

func (s Signal) String() string {
	if 0 <= s && int(s) < len(signals) {
		return signals[s]
	}

	return "signal " + strconv.Itoa(int(s))
}
