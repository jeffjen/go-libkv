// Package timer provides basic schedule primitives for user defined methods to
// be engaged after set period.
package timer

import (
	"container/heap"
	"fmt"
	"sync"
	"time"
)

const (
	IDLE int = iota
	STARTED
	STOPPED
)

// schedIdx is how we map job identifier to position in the queue.
type schedIdx struct {
	sync.RWMutex
	i map[int64]int
}

type Timer struct {
	begin int64 `desc: job identifier counter`

	inn    chan *ticket  `desc: input channel for incoming job ticket`
	sync   chan *ticket  `desc: input channel for update request`
	pq     priority      `desc: priority queue for the scheduler`
	halt   chan struct{} `desc: channel as stopping condition`
	end    chan struct{} `desc: channel to wait on stop completion`
	state  int           `desc: mark state of the Timer`
	lookup *schedIdx     `desc: lookup table from job id to priority queue index`
}

func (t *Timer) repeat(job *ticket) (ack bool) {
	t.lookup.Lock()
	defer t.lookup.Unlock()
	if _, ack = t.lookup.i[job.iden]; ack {
		job.a = time.Now().Add(job.d)
		heap.Push(&t.pq, job)
	}
	return
}

// pop removes an item for the dispatch lookup table.
func (t *Timer) pop(iden int64) (ack bool) {
	t.lookup.Lock()
	defer t.lookup.Unlock()
	if _, ack = t.lookup.i[iden]; ack {
		delete(t.lookup.i, iden)
	}
	return
}

// place adds one job to the Timer object.
func (t *Timer) place(job *ticket) {
	t.lookup.Lock()
	defer t.lookup.Unlock()
	t.lookup.i[job.iden] = job.pos
}

// dispatch finds expired timer and schedule job on its own goroutine.
func (t *Timer) dispatch() {
	now := time.Now()
	for ok := true; ok; {
		if len(t.pq) > 0 && now.After(t.pq[0].a) {
			job := heap.Pop(&t.pq).(*ticket)
			if job.r {
				if t.repeat(job) {
					go job.h.Done(job.iden) // fire the worker on timer
				}
			} else {
				if t.pop(job.iden) {
					go job.h.Done(job.iden) // fire the worker on timer
				}
			}
		} else {
			ok = false
		}
	}
}

// Tic starts the Timer in a goroutine.
// Accepts incoming schedudle request via SchedFunc and Sched.
func (t *Timer) Tic() {
	if t.state != IDLE {
		panic(fmt.Errorf("timer must be in IDLE to Tic"))
	} else {
		t.state = STARTED
	}

	t.halt, t.end = make(chan struct{}), make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		var alarm <-chan time.Time
		for yay := true; yay; {
			// determine the closest future about to happen
			if len(t.pq) == 0 {
				alarm = time.After(1 * time.Hour)
			} else {
				diff := t.pq[0].a.Sub(time.Now())
				if diff < 0 {
					t.dispatch()
					continue
				}
				alarm = time.After(diff)
			}

			select {
			case <-t.halt:
				yay = false // someone closed the light, quit

			case job := <-t.inn:
				heap.Push(&t.pq, job)
				t.place(job)

			case old_job := <-t.sync:
				heap.Fix(&t.pq, old_job.pos)
				t.place(old_job)

			case <-alarm:
				t.dispatch()
			}
		}
	}()

	go func() { // wait for timer runner and collect it
		wg.Wait()
		close(t.end)
	}()
}

// Toc stops the Timer and all schedudled handler are dropped.
// Stopped timer cannot be restarted.
func (t *Timer) Toc() {
	close(t.halt)
	for _ = range t.end {
	}
	t.state = STOPPED
}

// RepeatFunc accepts a duration as interval and a handle function.
// handle function is invoked in its own goroutine at set interval.
// Returns an identifier for caller to Cancel.
//
// CAUTION: if the interval is set lower relative to handle function, you will
// have an unbounded number of goroutine
func (t *Timer) RepeatFunc(d time.Duration, handle func(int64)) (iden int64) {
	t.begin, iden = t.begin+1, t.begin
	t.inn <- &ticket{
		a:    time.Now().Add(d),
		h:    HandlerFunc(handle),
		iden: iden,
		r:    true,
		d:    d,
	}
	return
}

// Repeat accepts a time.Duration and a Handler interface.
// handler is invoked in its own goroutine at set interval.
// Returns an identifier for caller to Cancel.
//
// CAUTION: if the interval is set lower relative to handle function, you will
// have an unbounded number of goroutine
func (t *Timer) Repeat(d time.Duration, handle Handler) (iden int64) {
	t.begin, iden = t.begin+1, t.begin
	t.inn <- &ticket{
		a:    time.Now().Add(d),
		h:    handle,
		iden: iden,
		r:    true,
		d:    d,
	}
	return
}

// SchedFunc accepts time.Time object and a handle function.
// handle function is invoked in its own goroutine at designated time.
// Returns an identifier for caller to Cancel or Update.
func (t *Timer) SchedFunc(c time.Time, handle func(int64)) (iden int64) {
	t.begin, iden = t.begin+1, t.begin
	t.inn <- &ticket{a: c, h: HandlerFunc(handle), iden: iden}
	return
}

// Sched accepts time.Time object and a Handler interface.
// handler is invoked in its own goroutine when at designated time.
// Returns an identifier for caller to Cancel or Update.
func (t *Timer) Sched(c time.Time, handle Handler) (iden int64) {
	t.begin, iden = t.begin+1, t.begin
	t.inn <- &ticket{a: c, h: handle, iden: iden}
	return
}

// Update takes an identifier by SchedFunc or Sched and reschedules its TTL.
func (t *Timer) Update(iden int64, c time.Time) {
	t.lookup.RLock()
	defer t.lookup.RUnlock()
	if pos, ok := t.lookup.i[iden]; ok {
		job := t.pq[pos]
		job.a = c
		t.sync <- job
	}
}

// Cancel takes an identifier by SchedFunc or Sched and disables the handler.
// Effectively prevents the handler to be invoked.
func (t *Timer) Cancel(iden int64) {
	t.pop(iden)
}

// NewTimer creates a Timer object.  Timer is initialized by not engaged.
// To enage the Timer, invoke Tic.
// To stop the Timer, invoke Toc.
func NewTimer() (t *Timer) {
	pri := make(priority, 0)
	heap.Init(&pri)
	return &Timer{
		begin:  1, // initialized to positive value
		state:  IDLE,
		inn:    make(chan *ticket, 1),
		sync:   make(chan *ticket, 1),
		pq:     pri,
		halt:   nil,
		end:    nil,
		lookup: &schedIdx{i: make(map[int64]int)},
	}
}
