# go-timer
[![Build Status](https://travis-ci.org/jeffjen/go-timer.svg?branch=master)](https://travis-ci.org/jeffjen/go-timer)

A framework for handling time based resource management

## timer

Schedule work to run at specific time.  It uses implementation in
[container/heap](http://golang.org/pkg/container/heap/) to setup min-heap on
the TTL of the scheduled item.

## session

Based on timer, I present a simple in memory volaile KV storage inspired by
[redis](http://redis.io/).

Supported operations

- Set, Setexp
- Get
- Getset
- Expire
- Del
- Watch
