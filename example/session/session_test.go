package session

import (
	"fmt"
	"testing"
	"time"
)

func TestSessionStore(t *testing.T) {
	sess := New()
	defer sess.Close()

	sess.Set("hello_internet", 1)
	sess.Set("hello_world", func() { fmt.Println("hello human") })

	x := sess.Get("hello_internet").(int)
	if x != 1 {
		t.Errorf("invalid value for stroed key world: \"hello_internet\"")
		return
	}

	sess.Del("hello_world")

	y := sess.Get("hello_world")
	if y != nil {
		t.Errorf("unable to delete stored key world: \"hello_world\"")
		return
	}
}

func TestExpire(t *testing.T) {
	sess := New()
	defer sess.Close()

	sess.Setexp("hello_internet", 1, time.Now().Add(1*time.Second))

	sess.Set("hello_world", 2)

	time.Sleep(2 * time.Second)

	x := sess.Get("hello_internet")
	if x != nil {
		t.Errorf("key world: \"hello_internet\" not expired")
		return
	}

	y := sess.Get("hello_world").(int)
	if y != 2 {
		t.Errorf("key world: \"hello_world\" unexpected get failure")
		return
	}

	if !sess.Expire("hello_world", time.Now().Add(1*time.Second)) {
		t.Errorf("key world: \"hello_world\" missing")
		return
	}

	time.Sleep(2 * time.Second)

	z := sess.Get("hello_world")
	if z != nil {
		t.Errorf("key world: \"hello_world\" not expired")
		return
	}
}

func TestGetSet(t *testing.T) {
	sess := New()
	defer sess.Close()

	sess.Setexp("hello_internet", 1, time.Now().Add(1*time.Second))

	sess.Getset("hello_internet", 2)

	time.Sleep(1 * time.Second)

	x := sess.Get("hello_internet")
	if x == nil {
		t.Errorf("key world: \"hello_internet\" unexpected expire")
		return
	}

	if x.(int) != 2 {
		t.Errorf("key world: \"hello_internet\" holds invalid value")
		return
	}
}
