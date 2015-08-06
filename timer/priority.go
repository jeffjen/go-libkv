package timer

import (
	"time"
)

type ticket struct {
	iden int64
	pos  int

	a time.Time
	h Handler
}

type priority []*ticket

func (pri priority) Len() int { return len(pri) }

func (pri priority) Less(i, j int) bool {
	return pri[i].a.Before(pri[j].a)
}

func (pri priority) Swap(i, j int) {
	pri[j], pri[i] = pri[i], pri[j]
	pri[i].pos = i
	pri[j].pos = j
}

func (pri *priority) Push(x interface{}) {
	n := len(*pri)
	t := x.(*ticket)
	t.pos = n
	*pri = append(*pri, t)
}

func (pri *priority) Pop() interface{} {
	old, n := *pri, len(*pri)
	t := old[n-1]
	t.pos = -1 // invalidate
	*pri = old[0 : n-1]
	return t
}
