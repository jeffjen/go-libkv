// Package libkv provides key value storage for embedded go application.
package libkv

import (
	"encoding/gob"
	"io"
	"os"
	p "path"
	"sync"
	"time"
)

const (
	KV_FILE     = "kv.db"
	KV_IDX_FILE = "kv.idx"
)

var (
	g sync.Mutex
)

func init() {
	gob.Register([]thing{})
	gob.Register(thing{})
}

func Load(path string) (*Store, error) {
	g.Lock()
	defer g.Unlock()

	var (
		s = NewStore() // initiate kv store object

		store map[string]thing
	)

	if fp, err := os.Open(p.Join(path, KV_FILE)); err != nil {
		return s, err
	} else {
		defer fp.Close()
		dec := gob.NewDecoder(fp)
		if drr := dec.Decode(&store); drr != nil && drr != io.EOF {
			return s, err
		}
	}

	// loaded kv object file
	s.m.store = store

	// proceed to restore state
	now := time.Now()
	for k, v := range s.m.store {
		if v.T != nil && now.After(*v.T) {
			delete(store, k)
		} else if v.T != nil {
			s.expire(k, *v.T)
		}
	}

	return s, nil
}

// Save stores the kv object and kv index object to designated path
// NOTE: this is an experimental feature.  Method best invoked in goroutine
func (s *Store) Save(path string) error {
	s.RLock()
	defer s.RUnlock()

	if fp, err := os.Create(p.Join(path, KV_FILE)); err != nil {
		return err
	} else {
		defer fp.Close()
		enc := gob.NewEncoder(fp)
		if err = enc.Encode(s.m.store); err != nil {
			return err
		}
	}

	return nil
}
