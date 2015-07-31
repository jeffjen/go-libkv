package timer

type HandlerFunc func()

func (e HandlerFunc) Done() {
	e()
}

type Handler interface {
	Done()
}
