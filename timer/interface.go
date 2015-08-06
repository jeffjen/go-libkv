package timer

// HandlerFunc is a wrapper type for standard function
type HandlerFunc func()

func (e HandlerFunc) Done() {
	e()
}

// Handler is the standard interface for Timer scheduler
type Handler interface {
	Done()
}
