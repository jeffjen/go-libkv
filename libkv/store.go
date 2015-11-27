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
	LPUSH
	LTRIM
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
	X interface{} `desc: the object to store`
	T *time.Time  `desc: the expiration date on this thing`
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
			src:     make(chan *Event, 8),
			list:    make(map[int64]*avent),
			halt:    make(chan struct{}),
			counter: 1, // initialzed to positive value
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

func (s *Store) pushEvent(event ...*Event) {
	for _, ev := range event {
		s.s.src <- ev
	}
}

// del removes an item from the Store keyspace
func (s *Store) del(iden string, jobId int64) bool {
	// check key exist
	if _, ok := s.m.store[iden]; !ok {
		return false
	}

	// check key job id; if exist then job id must match
	if jid, ok := s.m.index[iden]; ok && jobId != jid {
		// jobId will ALWAYS be positve value
		if jid == -1 {
			delete(s.m.index, iden)
		}
		return false
	}

	// remove key from store
	delete(s.m.store, iden)
	delete(s.m.index, iden)

	return true
}

// get retrieves an item x identified by iden
func (s *Store) get(iden string) (x interface{}) {
	if obj, ok := s.m.store[iden]; ok {
		x = obj.X
	}
	return
}

// set adds an item to the Store keyspace; sets expiration handler when
// appropriate
func (s *Store) set(iden string, x interface{}, exp *time.Time) bool {
	if idx, ok := s.m.index[iden]; ok {
		s.e.Cancel(idx)
		s.m.index[iden] = -1 // invalidate any fired expire handler
	}
	s.m.store[iden] = thing{X: x, T: exp}
	if exp != nil {
		s.expire(iden, *exp)
	}
	return true
}

func (s *Store) expire(iden string, exp time.Time) {
	id := iden
	s.m.index[iden] = s.e.SchedFunc(exp, func(jobId int64) {
		s.Lock()
		defer s.Unlock()
		if s.del(id, jobId) {
			s.pushEvent(&Event{GONE, iden})
		}
	})
}

// Set puts an aribtrary item x into Store identified by iden
func (s *Store) Set(iden string, x interface{}) (ret bool) {
	s.Lock()
	defer s.Unlock()
	ret = s.set(iden, x, nil)
	if ret {
		s.pushEvent(&Event{SET, iden})
	}
	return
}

// Set puts an aribtrary item x into Store identified by iden to be expired at
// exp
func (s *Store) Setexp(iden string, x interface{}, exp time.Time) (ret bool) {
	s.Lock()
	defer s.Unlock()
	ret = s.set(iden, x, &exp)
	if ret {
		s.pushEvent(&Event{SET, iden}, &Event{EXPIRE, iden})
	}
	return
}

// Get retrieves an item x identified by iden
func (s *Store) Get(iden string) (x interface{}) {
	s.RLock()
	defer s.RUnlock()
	if x = s.get(iden); x != nil {
		if _, ok := x.([]thing); !ok {
			s.pushEvent(&Event{GET, iden})
		} else {
			x = nil
		}
	}
	return
}

// Getset retrieves an item y identified by iden and replace it with item x
func (s *Store) Getset(iden string, x interface{}) (y interface{}) {
	s.Lock()
	defer s.Unlock()
	y = s.get(iden)
	s.set(iden, x, nil)
	s.pushEvent(&Event{GETSET, iden})
	return
}

// Getexp retrieves an item x identified by iden and set expiration
func (s *Store) Getexp(iden string, exp time.Time) (x interface{}) {
	s.Lock()
	defer s.Unlock()
	if x = s.get(iden); x != nil {
		s.pushEvent(&Event{GET, iden})
		s.expire(iden, exp)
	}
	return
}

// TTL reports the life time left on the item identified by iden
func (s *Store) TTL(iden string) (in time.Duration) {
	s.RLock()
	defer s.RUnlock()
	if obj, ok := s.m.store[iden]; ok && obj.T != nil {
		in = obj.T.Sub(time.Now())
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
		s.expire(iden, exp)
		s.pushEvent(&Event{EXPIRE, iden})
		return true
	}
}

// Del removes the item identified by iden from Store
func (s *Store) Del(iden string) {
	s.Lock()
	defer s.Unlock()
	jobId, _ := s.m.index[iden]
	if s.del(iden, jobId) {
		s.pushEvent(&Event{DEL, iden})
	}
}

// Lpush appends an item to item identified by iden
// creates new list item
// returns size of the item after operation; -1 for failed attempt
func (s *Store) Lpush(iden string, x interface{}) (size int64) {
	s.Lock()
	defer s.Unlock()
	if obj, ok := s.m.store[iden]; !ok {
		// the "thing" to store is a []thing with one new item
		s.m.store[iden] = thing{
			X: []thing{
				thing{X: x, T: nil},
			},
			T: nil,
		}
		size = 1
		s.pushEvent(&Event{LPUSH, iden})
	} else if lobj, ok := obj.X.([]thing); ok {
		// find the "thing", check that it is a []thing, and append to it
		lobj = append([]thing{thing{X: x, T: nil}}, lobj...)
		s.m.store[iden] = thing{X: lobj, T: obj.T}
		size = int64(len(lobj)) + 1
		s.pushEvent(&Event{LPUSH, iden})
	} else {
		size = -1
	}
	return
}

func (s *Store) rangeidx(start, stop, length int64) (begin, end int64) {
	if start >= length {
		begin = length
	} else if start >= 0 && start < length {
		begin = start
	} else {
		begin = length + start
		if begin < 0 {
			begin = 0
		}
	}
	if stop >= length {
		end = length
	} else if stop >= 0 && stop < length {
		end = stop
	} else {
		end = length + stop + 1
	}
	if end < begin {
		end = begin
	}
	return
}

// Lrange returns a slice of items within start and stop.
func (s *Store) Lrange(iden string, start, stop int64) (items []interface{}) {
	s.Lock()
	defer s.Unlock()
	if obj, ok := s.m.store[iden]; !ok {
		items = nil
	} else if lobj, ok := obj.X.([]thing); !ok {
		items = nil
	} else {
		begin, end := s.rangeidx(start, stop, int64(len(lobj)))
		for _, obj := range lobj[begin:end] {
			items = append(items, obj.X)
		}
	}
	return
}

// Ltrim keeps items specified in start and stop range and remove all other
// items.
// start and stop can be negative values.  If the value is -1, it indicates the
// end of list; if it is greater then the actual length, it is clamped to the
// boundary beteween 0 and length of item
// returns size of the item after operation; -1 for failed attempt
func (s *Store) Ltrim(iden string, start, stop int64) (size int64) {
	s.Lock()
	defer s.Unlock()
	if obj, ok := s.m.store[iden]; !ok {
		size = -1
	} else if lobj, ok := obj.X.([]thing); !ok {
		size = -1
	} else {
		begin, end := s.rangeidx(start, stop, int64(len(lobj)))
		lobj = lobj[begin:end]
		s.m.store[iden] = thing{X: lobj, T: obj.T}
		size = int64(len(lobj))
		s.pushEvent(&Event{LTRIM, iden})
	}
	return
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

// Key retrieves the full list of item key in Store.
func (s *Store) Key() (items []string) {
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

// Keyexp retrieves the full list of item key in Store that has an expiration.
func (s *Store) Keyexp() (items []string) {
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

// Iterate goes through the key set calles the provided handler with key and
// value
func (s *Store) IterateFunc(do func(string, interface{})) {
	s.RLock()
	defer s.RUnlock()
	for k, v := range s.m.store {
		do(k, v.X)
	}
}
