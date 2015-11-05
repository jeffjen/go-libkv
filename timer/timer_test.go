package timer

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

type Expire struct {
	iden int
	wg   *sync.WaitGroup
}

func (e Expire) Done(jobId int64) {
	fmt.Printf("triggered expired [%d] - [%d] - [%s]\n", e.iden, jobId, time.Now())
	e.wg.Done()
}

func TestSchedTimer(t *testing.T) {
	exp := NewTimer()

	exp.Tic()
	defer exp.Toc() // schedule expire worker

	var (
		wg sync.WaitGroup

		cases = []time.Duration{
			300 * time.Millisecond,
			5 * time.Second,
			1 * time.Second,
		}

		end = make(chan struct{})
	)

	wg.Add(len(cases))
	go func() {
		wg.Wait()
		close(end)
	}()

	now := time.Now()
	fmt.Printf("begin TestSchedTimer %v\n", now)

	for idx, c := range cases {
		exp.Sched(now.Add(c), Expire{idx, &wg})
	}

	select {
	case <-end:
		return
	case <-time.After(1 * time.Minute):
		t.Errorf("Unable to complete test")
	}
}

func TestBurstSched(t *testing.T) {
	exp := NewTimer()

	exp.Tic()
	defer exp.Toc() // schedule expire worker

	var (
		wg sync.WaitGroup

		schedNumHandler = 10000

		end = make(chan struct{})
	)

	wg.Add(schedNumHandler)
	go func() {
		wg.Wait()
		close(end)
	}()

	now := time.Now()
	fmt.Printf("begin TestBurstSched %v\n", now)

	for idx := 0; idx < schedNumHandler; idx += 1 {
		exp.SchedFunc(now.Add(1*time.Millisecond), func(jobId int64) {
			wg.Done()
		})
	}

	select {
	case <-end:
		return
	case <-time.After(time.Duration(schedNumHandler) * time.Millisecond):
		t.Errorf("Unable to complete test")
	}
}

func TestCancel(t *testing.T) {
	exp := NewTimer()

	exp.Tic()
	defer exp.Toc() // schedule expire worker

	now := time.Now()
	fmt.Printf("begin TestCancel %v\n", now)

	resp := make(chan int, 1)

	iden := exp.SchedFunc(now.Add(10*time.Second), func(id int64) {
		fmt.Printf("work order [%d;%d] triggered at %v\n", 1, id, time.Now())
		resp <- 1
	})

	exp.SchedFunc(now.Add(2*time.Second), func(id int64) {
		fmt.Printf("work order [%d;%d] triggered at %v\n", 2, id, time.Now())
		resp <- 2
	})

	woe := time.After(10 * time.Second)
	for {
		select {
		case v := <-resp:
			if v != 2 {
				t.Errorf("Failed to cancel work %d", v)
			} else {
				exp.Cancel(iden) // cancel the first one
			}
		case <-woe:
			return
		}
	}
}

func TestUpdate(t *testing.T) {
	exp := NewTimer()

	exp.Tic()
	defer exp.Toc() // schedule expire worker

	now := time.Now()
	fmt.Printf("begin TestUpdate %v\n", now)

	resp := make(chan int, 1)

	iden := exp.SchedFunc(now.Add(10*time.Second), func(jobId int64) {
		resp <- 1
	})

	trigger := time.After(2 * time.Second)
	woe := time.After(5 * time.Second)
	for {
		select {
		case <-resp:
			return
		case <-trigger:
			exp.Update(iden, time.Now().Add(1*time.Second))
		case <-woe:
			t.Errorf("Failed to update scheduled work")
		}
	}
}

func TestRepeat(t *testing.T) {
	exp := NewTimer()

	exp.Tic()
	defer exp.Toc() // schedule expire worker

	now := time.Now()
	fmt.Printf("begin TestRepeat %v\n", now)

	resp := make(chan int, 1)

	iden := exp.RepeatFunc(1*time.Second, func(jobId int64) {
		resp <- 1
	})

	history, counter := 0, 0

	pre := time.After(5 * time.Second)
	end := time.After(10 * time.Second)
	for {
		select {
		case <-resp:
			counter += 1
		case <-pre:
			exp.Cancel(iden)
			if counter == 0 {
				t.Errorf("Failed to repeat scheduled work")
			} else {
				fmt.Printf("work order repated %d times\n", counter)
			}
			history = counter
		case <-end:
			if history != counter {
				t.Errorf("Failed to cancel repeat work")
			}
			return
		}
	}
}

func BenchmarkSchedFunc(b *testing.B) {
	exp := NewTimer()

	exp.Tic()
	defer exp.Toc() // schedule expire worker

	const N = 10000

	handle := func(jobId int64) {}

	future := time.Now().Add(1 * time.Hour)

	for idx := 0; idx < N; idx++ {
		exp.SchedFunc(future, handle)
	}
}
