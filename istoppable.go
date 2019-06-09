package graceful_shutdown

type IStoppable interface {
	Stop() error
}
