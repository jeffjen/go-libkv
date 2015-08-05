package session

import (
	expire "github.com/jeffjen/go-timer"
	"sync"
	"time"
)

const (
	GET int = iota
	SET
	DEL
	EXPIRE
)

type Event struct {
	Action int
	Iden   string
}

type avent struct {
	c chan<- *Event
	h chan struct{}
}

type session_avent struct {
	sync.RWMutex

	src     chan *Event
	list    map[int64]*avent
	halt    chan struct{}
	counter int64
}

type Session struct {
	sync.RWMutex

	e *expire.Timer
	m struct {
		store map[string]interface{}
		index map[string]int64
	}
	s *session_avent
}

func (s *Session) init() {
	s.e.Tic()
	go func() {
		for yay := true; yay; {
			select {
			case <-s.s.halt:
				yay = false
			case one_event := <-s.s.src:
				s.s.Lock()
				for idx, ev := range s.s.list {
					select {
					case <-ev.h:
						delete(s.s.list, idx)
					case ev.c <- one_event:
						continue
					}
				}
				s.s.Unlock()
			}
		}
	}()
}

func New() (s *Session) {
	s = &Session{
		e: expire.NewTimer(),
		m: struct {
			store map[string]interface{}
			index map[string]int64
		}{
			store: make(map[string]interface{}),
			index: make(map[string]int64),
		},
		s: &session_avent{
			src:  make(chan *Event, 1),
			list: make(map[int64]*avent),
			halt: make(chan struct{}),
		},
	}
	s.init()
	return
}

func (s *Session) Close() {
	close(s.s.halt)
	s.e.Toc()
}

func (s *Session) del(iden string) {
	s.Lock()
	defer s.Unlock()
	delete(s.m.store, iden)
	delete(s.m.index, iden)
	s.s.src <- &Event{DEL, iden}
}

func (s *Session) set(iden string, x interface{}, exp *time.Time) bool {
	s.Lock()
	defer s.Unlock()
	if idx, ok := s.m.index[iden]; ok {
		s.e.Cancel(idx)
	}
	s.m.store[iden] = x
	if exp != nil {
		id := iden
		s.m.index[iden] = s.e.SchedFunc(*exp, func() {
			s.del(id)
			s.s.src <- &Event{EXPIRE, iden}
		})
	}
	s.s.src <- &Event{SET, iden}
	return true
}

func (s *Session) Set(iden string, x interface{}) bool {
	return s.set(iden, x, nil)
}

func (s *Session) Setexp(iden string, x interface{}, exp time.Time) bool {
	return s.set(iden, x, &exp)
}

func (s *Session) Get(iden string) (x interface{}) {
	s.RLock()
	defer s.RUnlock()
	x = s.m.store[iden]
	s.s.src <- &Event{GET, iden}
	return
}

func (s *Session) Getset(iden string, x interface{}) (y interface{}) {
	y = s.Get(iden)
	s.Set(iden, x)
	return
}

func (s *Session) Expire(iden string, exp time.Time) bool {
	x := s.Get(iden)
	if x != nil {
		return s.Setexp(iden, x, exp)
	}
	return false
}

func (s *Session) Del(iden string) {
	s.del(iden)
}

func (s *Session) register(inn *avent) {
	s.s.Lock()
	defer s.s.Unlock()
	r := s.s.counter
	s.s.counter = s.s.counter + 1
	s.s.list[r] = inn
}

func (s *Session) Watch(stop chan struct{}) <-chan *Event {
	output := make(chan *Event, 8)
	go func() {
		defer close(output)
		incoming := make(chan *Event, 1)
		end := make(chan struct{})
		s.register(&avent{incoming, end})
		defer close(end)
		for yay := true; yay; {
			select {
			case <-stop:
				yay = false
			case ev := <-incoming:
				select {
				default:
					yay = false
				case output <- ev:
					continue
				}
			}
		}
	}()
	return output
}
