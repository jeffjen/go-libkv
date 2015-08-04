package session

import (
	expire "github.com/jeffjen/go-timer"
	"sync"
	"time"
)

type Session struct {
	sync.RWMutex
	e *expire.Timer
	m struct {
		store map[string]interface{}
		index map[string]int64
	}
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
	}
	s.e.Tic()
	return
}

func (s *Session) Close() {
	s.e.Toc()
}

func (s *Session) del(iden string) {
	s.Lock()
	defer s.Unlock()
	delete(s.m.store, iden)
	delete(s.m.index, iden)
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
		s.m.index[iden] = s.e.SchedFunc(*exp, func() { s.del(id) })
	}
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
