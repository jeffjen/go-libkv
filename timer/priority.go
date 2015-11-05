package timer

import (
	"time"
)

// ticket is job metadata.
type ticket struct {
	iden int64 `desc: uniqe identifier for a job`
	pos  int   `desc: position in the priority queue`

	a time.Time `desc: the designated time to fire Handler`
	h Handler   `desc: registered handler`

	r       bool          `desc: flag for testing whether this is a recurring ticket`
	d       time.Duration `desc: the interval at which we repeat this ticket`
	concurr int           `desc: the maximum number of concurrent task to run`
	ntask   int           `desc: the number of task running right now`
}

// priority is how we manage scheduling in Timer
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
