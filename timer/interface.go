package timer

// HandlerFunc is a wrapper type for standard function
type HandlerFunc func(int64)

func (e HandlerFunc) Done(jobId int64) {
	e(jobId)
}

// Handler is the standard interface for Timer scheduler
type Handler interface {
	Done(jobId int64)
}
