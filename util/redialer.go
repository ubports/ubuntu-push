/*
 Copyright 2013-2014 Canonical Ltd.

 This program is free software: you can redistribute it and/or modify it
 under the terms of the GNU General Public License version 3, as published
 by the Free Software Foundation.

 This program is distributed in the hope that it will be useful, but
 WITHOUT ANY WARRANTY; without even the implied warranties of
 MERCHANTABILITY, SATISFACTORY QUALITY, or FITNESS FOR A PARTICULAR
 PURPOSE.  See the GNU General Public License for more details.

 You should have received a copy of the GNU General Public License along
 with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package util

import (
	"math/rand"
	"sync"
	"time"
)

// A Dialer is an object that knows how to establish a connection, and
// where you'd usually want some kind of back off if that connection
// fails.
type Dialer interface {
	Dial() error
	String() string
	Jitter(time.Duration) time.Duration
}

// The timeouts used during backoff.
var timeouts []time.Duration
var trwlock sync.RWMutex

var ( //  for use in testing
	quitRedialing chan bool = make(chan bool)
)

func Timeouts() []time.Duration {
	trwlock.RLock()
	defer trwlock.RUnlock()
	return timeouts
}

// for testing
func SwapTimeouts(newTimeouts []time.Duration) (oldTimeouts []time.Duration) {
	trwlock.Lock()
	defer trwlock.Unlock()
	oldTimeouts, timeouts = timeouts, newTimeouts
	return
}

// Jitter returns a random time.Duration somewhere in [-spread, spread].
//
// This is meant as a default implementation for Dialers to use if wanted.
func Jitter(spread time.Duration) time.Duration {
	if spread < 0 {
		panic("spread must be non-negative")
	}
	n := int64(spread)
	return time.Duration(rand.Int63n(2*n+1) - n)
}

type AutoRetrier struct {
	Stop   chan bool
	Dial   func() error
	Jitter func(time.Duration) time.Duration
}

// AutoRetry keeps on calling Dial until it stops returning an error.  It does
// exponential backoff, adding the output of Jitter at each step back.
func (ar *AutoRetrier) Retry() uint32 {
	var timeout time.Duration
	var dialAttempts uint32 = 0 // unsigned so it can wrap safely ...
	timeouts := Timeouts()
	var numTimeouts uint32 = uint32(len(timeouts))
	for {
		if ar.Dial() == nil {
			return dialAttempts + 1
		}
		if dialAttempts < numTimeouts {
			timeout = timeouts[dialAttempts]
		} else {
			timeout = timeouts[numTimeouts-1]
		}
		timeout += ar.Jitter(timeout)
		dialAttempts++
		select {
		case <-ar.Stop:
			return dialAttempts
		case <-time.NewTimer(timeout).C:
		}
	}
}

// AutoRedialer takes a Dialer and retries its Dial() method until it
// stops returning an error. It does exponential (optionally
// jitter'ed) backoff.
func AutoRedial(dialer Dialer) uint32 {
	ar := &AutoRetrier{quitRedialing, dialer.Dial, dialer.Jitter}
	return ar.Retry()
}

func init() {
	ps := []int{1, 2, 5, 11, 19, 37, 67, 113, 191} // 3 pₙ₊₁ ≥ 5 pₙ
	timeouts := make([]time.Duration, len(ps))
	for i, n := range ps {
		timeouts[i] = time.Duration(n) * time.Second
	}
	SwapTimeouts(timeouts)
	rand.Seed(time.Now().Unix()) // good enough for us (not crypto, yadda)
}
