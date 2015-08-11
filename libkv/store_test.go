package libkv

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestStore(t *testing.T) {
	kv := NewStore()
	defer kv.Close()

	kv.Set("hello_internet", 1)
	kv.Set("hello_world", func() { fmt.Println("hello human") })

	x := kv.Get("hello_internet").(int)
	if x != 1 {
		t.Errorf("invalid value for stroed key word: \"hello_internet\"")
		return
	}

	kv.Del("hello_world")

	y := kv.Get("hello_world")
	if y != nil {
		t.Errorf("unable to delete stored key word: \"hello_world\"")
		return
	}
}

func TestExpire(t *testing.T) {
	kv := NewStore()
	defer kv.Close()

	kv.Setexp("hello_internet", 1, time.Now().Add(1*time.Second))

	kv.Set("hello_world", 2)

	time.Sleep(2 * time.Second)

	x := kv.Get("hello_internet")
	if x != nil {
		t.Errorf("key word: \"hello_internet\" not expired")
		return
	}

	y := kv.Get("hello_world").(int)
	if y != 2 {
		t.Errorf("key word: \"hello_world\" unexpected get failure")
		return
	}

	if !kv.Expire("hello_world", time.Now().Add(1*time.Second)) {
		t.Errorf("key word: \"hello_world\" missing")
		return
	}

	time.Sleep(2 * time.Second)

	z := kv.Get("hello_world")
	if z != nil {
		t.Errorf("key word: \"hello_world\" not expired")
		return
	}
}

func TestGetSet(t *testing.T) {
	kv := NewStore()
	defer kv.Close()

	kv.Setexp("hello_internet", 1, time.Now().Add(1*time.Second))

	kv.Getset("hello_internet", 2)

	time.Sleep(1 * time.Second)

	x := kv.Get("hello_internet")
	if x == nil {
		t.Errorf("key word: \"hello_internet\" unexpected expire")
		return
	}

	if x.(int) != 2 {
		t.Errorf("key word: \"hello_internet\" holds invalid value")
		return
	}
}

func TestAcquireTTL(t *testing.T) {
	kv := NewStore()
	defer kv.Close()

	kv.Setexp("hello_internet", 1, time.Now().Add(5*time.Second))

	kv.Set("hello_world", 1)

	if kv.TTL("hello_internet") == 0 {
		t.Errorf("key word: \"hello_internet\" holds invalid expire time")
		return
	}

	if kv.TTL("hello_world") != 0 {
		t.Errorf("key word: \"hello_world\" holds expire time")
	}
}

func TestList(t *testing.T) {
	kv := NewStore()
	defer kv.Close()

	const N = 10

	for idx := 0; idx < N; idx++ {
		kv.Set(fmt.Sprint(idx), idx)
	}

	k := kv.List()
	if len(k) != N {
		t.Errorf("unable to list keys stored")
	} else {
		fmt.Println(k)
	}
}

func TestListexp(t *testing.T) {
	kv := NewStore()
	defer kv.Close()

	const N = 100

	var exp_k = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}

	now := time.Now().Add(2 * time.Second)

	for idx, k := range exp_k {
		kv.Setexp(k, idx, now)
	}
	for idx := N; idx < 10+N; idx++ {
		kv.Set(fmt.Sprint(idx), idx)
	}

	k := kv.Listexp()

	sort.Strings(k)

	if !reflect.DeepEqual(k, exp_k) {
		t.Errorf("unexpected exp keys mismatch")
	} else {
		fmt.Println(k)
	}
}

func TestWatch(t *testing.T) {
	kv := NewStore()
	defer kv.Close()

	stop := make(chan struct{})
	defer close(stop)

	// set stuff randomly before monitor
	kv.Set("hello_world", 2)
	kv.Getset("hello_world", 2)
	kv.Setexp("hello_world", 2, time.Now().Add(1*time.Second))
	kv.Set("hello_world", 2)

	p1 := make(chan int, 1)
	go func() {
		monitor := kv.Watch(stop)
		p1 <- 1
		for _ = range monitor {
			p1 <- 1
		}
	}()

	p2 := make(chan int, 1)
	go func() {
		monitor := kv.Watch(stop)
		p2 <- 1
		for _ = range monitor {
			p2 <- 1
		}
	}()

	_, _ = <-p1, <-p2 // wait for monitor process

	kv.Set("hello_world", 2)

	kv.Setexp("hello_internet", 1, time.Now().Add(1*time.Second))

	kv.Get("hello_internet")

	kv.Getset("hello_world", 3)

	woe := time.After(5 * time.Second)
	for idx1, ok1, idx2, ok2 := 0, true, 0, true; ok1 || ok2; {
		select {
		case <-p1:
			idx1 += 1
			ok1 = !(idx1 == 6)
		case <-p2:
			idx2 += 1
			ok2 = !(idx2 == 6)

		case <-woe:
			t.Errorf("unable to complete: expected events incomplete")
			return
		}
	}
}

func TestWatchExtended(t *testing.T) {
	kv := NewStore()
	defer kv.Close()

	stop := make(chan struct{})
	p1 := make(chan int, 1)
	go func() {
		monitor := kv.Watch(stop)
		p1 <- 1
		for _ = range monitor {
			p1 <- 1
		}
	}()

	later := make(chan struct{})
	p2 := make(chan int, 1)
	go func() {
		monitor := kv.Watch(later)
		p2 <- 1
		for _ = range monitor {
			p2 <- 1
		}
	}()

	_, _ = <-p1, <-p2 // wait for monitor process

	kv.Set("hello_world", 2)

	kv.Setexp("hello_internet", 1, time.Now().Add(1*time.Second))

	kv.Get("hello_internet")

	kv.Getset("hello_world", 3)

	close(stop) // kill the firs monitor prematruely

	woe := time.After(5 * time.Second)
	for idx1, idx2, ok := 0, 0, true; ok; {
		select {
		case <-p1:
			idx1 += 1
			if idx1 > 4 {
				t.Errorf("monitor 1 returned more then expected")
				return
			}

		case <-p2:
			idx2 += 1
			ok = !(idx2 == 6)

		case <-woe:
			t.Errorf("unable to complete: expected events incomplete")
			return
		}
	}

	close(later) // kill the second monitor
}

func TestWatchNotpresent(t *testing.T) {
	kv := NewStore()
	defer kv.Close()

	stop := make(chan struct{})
	p1 := make(chan *Event, 1)
	go func() {
		monitor := kv.Watch(stop)
		p1 <- nil
		for evt := range monitor {
			p1 <- evt
		}
		close(p1)
	}()

	<-p1 // oberver ready

	hits := make(chan []int, 1)
	go func() {
		evts := make([]int, 0)
		for c := range p1 {
			evts = append(evts, c.Action)
		}
		hits <- evts
	}()

	events := []int{SET, GET, DEL}

	kv.Get("where")
	kv.Set("hello", 1)
	kv.Del("where")
	kv.Get("hello")
	kv.Del("hello")

	time.Sleep(250 * time.Millisecond)
	close(stop)

	if evts := <-hits; !reflect.DeepEqual(events, evts) {
		t.Errorf("uexpected event set mismatch")
	}
}
