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

type schedIdx struct {
	sync.RWMutex
	i map[int64]int
}

type Timer struct {
	begin int64

	inn    chan *ticket
	sync   chan *ticket
	pq     priority
	halt   chan struct{}
	end    chan struct{}
	state  int
	lookup *schedIdx
}

func (t *Timer) pop(iden int64) (ack bool) {
	t.lookup.Lock()
	defer t.lookup.Unlock()
	if _, ack = t.lookup.i[iden]; ack {
		delete(t.lookup.i, iden)
	}
	return
}

func (t *Timer) place(job *ticket) {
	t.lookup.Lock()
	defer t.lookup.Unlock()
	t.lookup.i[job.iden] = job.pos
}

func (t *Timer) dispatch() {
	now := time.Now()
	for ok := true; ok; {
		if len(t.pq) > 0 && now.After(t.pq[0].a) {
			job := heap.Pop(&t.pq).(*ticket)
			if t.pop(job.iden) {
				go job.h.Done() // fire the worker on timer
			}
		} else {
			ok = false
		}
	}
}

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
		for yay := true; yay; {
			// determine the closest future about to happen
			var alarm <-chan time.Time
			if len(t.pq) == 0 {
				alarm = time.After(1 * time.Hour)
			} else {
				diff := t.pq[0].a.Sub(time.Now())
				if diff < 0 {
					t.dispatch()
					continue
				}
				alarm = time.Tick(t.pq[0].a.Sub(time.Now()))
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

func (t *Timer) Toc() {
	close(t.halt)
	for _ = range t.end {
	}
	t.state = STOPPED
}

func (t *Timer) SchedFunc(c time.Time, handle func()) (iden int64) {
	t.begin, iden = t.begin+1, t.begin
	t.inn <- &ticket{a: c, h: HandlerFunc(handle), iden: iden}
	return
}

func (t *Timer) Sched(c time.Time, handle Handler) (iden int64) {
	t.begin, iden = t.begin+1, t.begin
	t.inn <- &ticket{a: c, h: handle, iden: iden}
	return
}

func (t *Timer) Update(iden int64, c time.Time) {
	t.lookup.RLock()
	defer t.lookup.RUnlock()
	if pos, ok := t.lookup.i[iden]; ok {
		job := t.pq[pos]
		job.a = c
		t.sync <- job
	}
}

func (t *Timer) Cancel(iden int64) {
	t.pop(iden)
}

func NewTimer() (t *Timer) {
	pri := make(priority, 0)
	heap.Init(&pri)
	return &Timer{
		state:  IDLE,
		inn:    make(chan *ticket, 1),
		sync:   make(chan *ticket, 1),
		pq:     pri,
		halt:   nil,
		end:    nil,
		lookup: &schedIdx{i: make(map[int64]int)},
	}
}
