package libkv

import (
	expire "github.com/jeffjen/go-libkv/timer"
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

type kv_avent struct {
	sync.RWMutex

	src     chan *Event
	list    map[int64]*avent
	halt    chan struct{}
	counter int64
}

type thing struct {
	x interface{}
	t *time.Time
}

type Store struct {
	sync.RWMutex

	e *expire.Timer
	m struct {
		store map[string]thing
		index map[string]int64
	}
	s *kv_avent
}

func (s *Store) init() {
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

func NewStore() (s *Store) {
	s = &Store{
		e: expire.NewTimer(),
		m: struct {
			store map[string]thing
			index map[string]int64
		}{
			store: make(map[string]thing),
			index: make(map[string]int64),
		},
		s: &kv_avent{
			src:  make(chan *Event, 1),
			list: make(map[int64]*avent),
			halt: make(chan struct{}),
		},
	}
	s.init()
	return
}

func (s *Store) Close() {
	close(s.s.halt)
	s.e.Toc()
}

func (s *Store) del(iden string) {
	s.Lock()
	defer s.Unlock()
	delete(s.m.store, iden)
	delete(s.m.index, iden)
	s.s.src <- &Event{DEL, iden}
}

func (s *Store) set(iden string, x interface{}, exp *time.Time) bool {
	s.Lock()
	defer s.Unlock()
	if idx, ok := s.m.index[iden]; ok {
		s.e.Cancel(idx)
	}
	s.m.store[iden] = thing{x: x, t: exp}
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

func (s *Store) Set(iden string, x interface{}) bool {
	return s.set(iden, x, nil)
}

func (s *Store) Setexp(iden string, x interface{}, exp time.Time) bool {
	return s.set(iden, x, &exp)
}

func (s *Store) Get(iden string) (x interface{}) {
	s.RLock()
	defer s.RUnlock()
	if obj, ok := s.m.store[iden]; ok {
		x = obj.x
		s.s.src <- &Event{GET, iden}
	}
	return
}

func (s *Store) Getset(iden string, x interface{}) (y interface{}) {
	y = s.Get(iden)
	s.Set(iden, x)
	return
}

func (s *Store) TTL(iden string) (in time.Duration) {
	s.RLock()
	defer s.RUnlock()
	if obj, ok := s.m.store[iden]; ok && obj.t != nil {
		in = obj.t.Sub(time.Now())
		if in < 0 {
			in = time.Duration(0)
		}
	}
	return
}

func (s *Store) Expire(iden string, exp time.Time) bool {
	x := s.Get(iden)
	if x != nil {
		return s.Setexp(iden, x, exp)
	}
	return false
}

func (s *Store) Del(iden string) {
	s.del(iden)
}

func (s *Store) register(inn *avent) {
	s.s.Lock()
	defer s.s.Unlock()
	r := s.s.counter
	s.s.counter = s.s.counter + 1
	s.s.list[r] = inn
}

func (s *Store) Watch(stop chan struct{}) <-chan *Event {
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

func (s *Store) List() (items []string) {
	s.RLock()
	defer s.RUnlock()
	items = make([]string, len(s.m.store))
	idx := 0
	for k, _ := range s.m.store {
		items[idx] = k
		idx++
	}
	return
}

func (s *Store) Listexp() (items []string) {
	s.RLock()
	defer s.RUnlock()
	items = make([]string, len(s.m.index))
	idx := 0
	for k, _ := range s.m.index {
		items[idx] = k
		idx++
	}
	return
}
