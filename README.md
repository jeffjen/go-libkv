# go-libkv
[![license](http://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/jeffjen/go-libkv/master/LICENSE)
[![GoDoc](https://godoc.org/github.com/jeffjen/go-libkv?status.png)](https://godoc.org/github.com/jeffjen/go-libkv)
[![Build Status](https://travis-ci.org/jeffjen/go-libkv.svg?branch=master)](https://travis-ci.org/jeffjen/go-libkv)
[![Gitter](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/jeffjen/go-libkv?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge)

An embedded framework for handling key value written in pure golang.

- Provides a straight forward interface for handling *volatile* key value storage
- Added flavor for time based resource management (EXPIRE, TTL)
- Broadcast key space event to subscribed goroutines.

## libkv

Embedded In memory **volatile** KV storage inspired by [REDIS](http://redis.io/).

### Supported operations

- Set, Setexp
- Get
- Getset
- Getexp
- Expire
- TTL
- Del
- Lpush
- Ltrim
- Lrange
- List, Listexp
- Watch

### Experimental feature:
Snapshot creation truncates current kv file object, so no version support.

- Save  
    Takes a snapshot of the current kv object to disk.  The rule of encoding
follows golang package [gob](https://golang.org/pkg/encoding/gob/).
- Load  
    Loads snapshot from disk.

- IterateR  
    Move alone the keyspace and retrieve key value

- IterateW  
    Move alone the keyspace and send modify instructions along

## timer

Schedule work to run at specific time once, or repeat the task at set interval.
It uses implementation in [container/heap](http://golang.org/pkg/container/heap/)
to setup min-heap on the TTL of the scheduled item.
