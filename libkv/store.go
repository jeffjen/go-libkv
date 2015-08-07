// Package libkv provides key value storage for embedded go application.
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
	GONE
	GETSET
)

// Event delivers keyspace changes to registerd Watcher
type Event struct {
	Action int    `desc: action taken e.g. GET, SET, EXPIRE`
	Iden   string `desc: key that was affected`
}

// avent is a control object for Store event hub to deliver Event
type avent struct {
	c chan<- *Event   `desc: channel to send the event into`
	h <-chan struct{} `desc: control channel to indicate unregister`
}

// kv_avent is holds metadata about how to distribute keyspace Event
type kv_avent struct {
	sync.RWMutex

	src     chan *Event      `desc: Event source channel from keyspace`
	list    map[int64]*avent `desc: list of registerd party for Event`
	halt    chan struct{}    `desc: control channel to stop Event distribution`
	counter int64            `desc: counter to tag Watcher`
}

// thing is an object stored in Store
type thing struct {
	x interface{} `desc: the object to store`
	t *time.Time  `desc: the expiration date on this thing`
}

// store is the actual KV store
type store struct {
	store map[string]thing `desc: the actual KV store`
	index map[string]int64 `desc: the index mapper for key to schdule index`
}

// Store is a simple key value in memory storage.
// Upon initialization, Store provides general get, set, del, list operations,
// as well as the ability to expire a key and watch a key change.
type Store struct {
	sync.RWMutex

	e *expire.Timer `desc: scheduler for keyspace expiration`
	m *store        `desc: the actual store`
	s *kv_avent     `desc: keyspace event hub`
}

func (s *Store) event_hub() (ok <-chan bool) {
	ack := make(chan bool, 1)
	go func() {
		ack <- true
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
	return ack
}

// init setups the key value storage.
// Upon initialization, the Store spawns expiration scheduler and keyspace event hub.
func (s *Store) init() (ok <-chan bool) {
	s.e.Tic()
	return s.event_hub()
}

// NewStore creates a Store object.
// Store object is fully initialzed upon creation.
func NewStore() (s *Store) {
	s = &Store{
		e: expire.NewTimer(),
		m: &store{
			store: make(map[string]thing),
			index: make(map[string]int64),
		},
		s: &kv_avent{
			src:  make(chan *Event, 1),
			list: make(map[int64]*avent),
			halt: make(chan struct{}),
		},
	}
	<-s.init() // make sure both scheduler and event hub is running
	return
}

// Close stops the Store scheduler and event hub.
func (s *Store) Close() {
	close(s.s.halt)
	s.e.Toc()
}

// del removes an item from the Store keyspace
func (s *Store) del(iden string) {
	s.Lock()
	defer s.Unlock()
	delete(s.m.store, iden)
	delete(s.m.index, iden)
}

// set adds an item to the Store keyspace; sets expiration handler when
// appropriate
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
			s.s.src <- &Event{GONE, iden}
			s.del(id)
		})
		s.s.src <- &Event{EXPIRE, iden}
	}
	return true
}

// Set puts an aribtrary item x into Store identified by iden
func (s *Store) Set(iden string, x interface{}) (ret bool) {
	ret = s.set(iden, x, nil)
	if ret {
		s.s.src <- &Event{SET, iden}
	}
	return
}

// Set puts an aribtrary item x into Store identified by iden to be expired at
// exp
func (s *Store) Setexp(iden string, x interface{}, exp time.Time) (ret bool) {
	ret = s.set(iden, x, &exp)
	if ret {
		s.s.src <- &Event{SET, iden}
	}
	return
}

func (s *Store) get(iden string) (x interface{}) {
	s.RLock()
	defer s.RUnlock()
	if obj, ok := s.m.store[iden]; ok {
		x = obj.x
	}
	return
}

// Get retrieves an item x identified by iden
func (s *Store) Get(iden string) (x interface{}) {
	x = s.get(iden)
	if x != nil {
		s.s.src <- &Event{GET, iden}
	}
	return
}

// Getset retrieves an item y identified by iden and replace it with item x
func (s *Store) Getset(iden string, x interface{}) (y interface{}) {
	y = s.get(iden)
	s.set(iden, x, nil)
	s.s.src <- &Event{GETSET, iden}
	return
}

// TTL reports the life time left on the item identified by iden
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

// Expire puts item identified by iden to expire at exp
func (s *Store) Expire(iden string, exp time.Time) bool {
	s.Lock()
	defer s.Unlock()

	if _, ok := s.m.store[iden]; !ok {
		return false
	} else {
		id := iden
		s.m.index[iden] = s.e.SchedFunc(exp, func() {
			s.s.src <- &Event{GONE, iden}
			s.del(id)
		})
		s.s.src <- &Event{EXPIRE, iden}
		return true
	}
}

// Del removes the item identified by iden from Store
func (s *Store) Del(iden string) {
	s.del(iden)
	s.s.src <- &Event{DEL, iden}
}

// register takes a control object avent and place it into the Wather list in
// keyspace event hub.
func (s *Store) register(inn *avent) {
	s.s.Lock()
	defer s.s.Unlock()
	r := s.s.counter
	s.s.counter = s.s.counter + 1
	s.s.list[r] = inn
}

// Watch provides interested party to monitor keyspace changes.
// A Watcher (caller of Watch function) provides a stopping condition, and gets
// a channel for future Event in keyspace.
// Stop a Watcher by closeing the stop channel
func (s *Store) Watch(stop <-chan struct{}) <-chan *Event {
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

// List retrieves the full list of item key in Store.
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

// Listexp retrieves the full list of item key in Store that has an expiration.
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
