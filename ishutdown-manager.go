package graceful_shutdown

import (
	"os"
)

type IShutdownManager interface {
	AppendServices(...IStoppable)
	RegisterSignals(...os.Signal) error
	ResetSignals(...os.Signal)
	TriggerShutdown()
	TriggerFatal()
	Wait() int
}
