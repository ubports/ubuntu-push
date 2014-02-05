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
	"sync"
	"time"
)

// A Dialer is an object that knows how to establish a connection, and
// where you'd usually want some kind of back off if that connection
// fails.
type Dialer interface {
	Dial() error
}

// A Jitterer is a Dialer that wants to vary the backoff a little (to avoid a
// thundering herd, for example).
type Jitterer interface {
	Dialer
	Jitter(time.Duration) time.Duration
}

// The timeouts used during backoff.
var timeouts []time.Duration
var trwlock sync.RWMutex

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

// An AutoRedialer's Redial() method retries its dialer's Dial() method until
// it stops returning an error. It does exponential (optionally jitter'ed)
// backoff.
type AutoRedialer interface {
	Redial() uint32
	Stop()
}

type autoRedialer struct {
	stop   chan bool
	lock   sync.RWMutex
	dial   func() error
	jitter func(time.Duration) time.Duration
}

// Stop shuts down the given AutoRedialer, if it is still retrying.
func (ar *autoRedialer) Stop() {
	if ar != nil {
		ar.lock.RLock()
		defer ar.lock.RUnlock()
		if ar.stop != nil {
			ar.stop <- true
		}
	}
}

func (ar *autoRedialer) shutdown() {
	ar.lock.Lock()
	defer ar.lock.Unlock()
	close(ar.stop)
	ar.stop = nil
}

// Redial keeps on calling Dial until it stops returning an error.  It does
// exponential backoff, adding the output of Jitter at each step back.
func (ar *autoRedialer) Redial() uint32 {
	if ar == nil {
		// at least it's better than the segfault...
		panic("you can't Redial a nil AutoRedialer")
	}
	if ar.stop == nil {
		panic("this AutoRedialer has already been shut down")
	}
	defer ar.shutdown()

	ar.lock.RLock()
	stop := ar.stop
	ar.lock.RUnlock()

	var timeout time.Duration
	var dialAttempts uint32 = 0 // unsigned so it can wrap safely ...
	timeouts := Timeouts()
	var numTimeouts uint32 = uint32(len(timeouts))
	for {
		if ar.dial() == nil {
			return dialAttempts + 1
		}
		if dialAttempts < numTimeouts {
			timeout = timeouts[dialAttempts]
		} else {
			timeout = timeouts[numTimeouts-1]
		}
		if ar.jitter != nil {
			timeout += ar.jitter(timeout)
		}
		dialAttempts++
		select {
		case <-stop:
			return dialAttempts
		case <-time.After(timeout):
		}
	}
}

// returns a stoppable AutoRedialer using the provided Dialer. If the Dialer
// is also a Jitterer, the backoff will be jittered.
func NewAutoRedialer(dialer Dialer) AutoRedialer {
	ar := &autoRedialer{stop: make(chan bool), dial: dialer.Dial}
	jitterer, ok := dialer.(Jitterer)
	if ok {
		ar.jitter = jitterer.Jitter
	}
	return ar
}

func init() {
	ps := []int{1, 2, 5, 11, 19, 37, 67, 113, 191} // 3 pₙ₊₁ ≥ 5 pₙ
	timeouts := make([]time.Duration, len(ps))
	for i, n := range ps {
		timeouts[i] = time.Duration(n) * time.Second
	}
	SwapTimeouts(timeouts)
}
