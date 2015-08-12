# go-libkv
[![license](http://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/jeffjen/go-libkv/master/LICENSE)
[![GoDoc](https://godoc.org/github.com/jeffjen/go-libkv?status.png)](https://godoc.org/github.com/jeffjen/go-libkv)
[![Build Status](https://travis-ci.org/jeffjen/go-libkv.svg?branch=master)](https://travis-ci.org/jeffjen/go-libkv)

An embedded framework for handling key value written in pure golang.

- Provides a straight forward interface for handling *volatile* key value storage
- Added flavor for time based resource management (EXPIRE, TTL)
- Broadcast key space event to subscribed goroutines.

## libkv

Embedded In memory **volatile** KV storage inspired by [REDIS](http://redis.io/).

Supported operations

- Set, Setexp
- Get
- Getset
- Getexp
- Expire
- TTL
- Del
- List, Listexp
- Watch

## timer

Schedule work to run at specific time.  It uses implementation in
[container/heap](http://golang.org/pkg/container/heap/) to setup min-heap on
the TTL of the scheduled item.
