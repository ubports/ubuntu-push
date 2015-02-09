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

// Package util contains the redialer.
package util

import (
	"sync"
	"time"
)

// A Dialer is an object that knows how to establish a connection, and
// where you'd usually want some kind of backoff if that connection
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

// Retrieve the list of timeouts used for exponential backoff.
func Timeouts() []time.Duration {
	trwlock.RLock()
	defer trwlock.RUnlock()
	return timeouts
}

// For testing: change the default timeouts with the provided ones,
// returning the defaults (the idea being you reset them on test
// teardown).
func SwapTimeouts(newTimeouts []time.Duration) (oldTimeouts []time.Duration) {
	trwlock.Lock()
	defer trwlock.Unlock()
	oldTimeouts, timeouts = timeouts, newTimeouts
	return
}

// An AutoRedialer's Redial() method retries its dialer's Dial() method until
// it stops returning an error. It does exponential backoff (optionally
// jittered).
type AutoRedialer interface {
	Redial() uint32 // Redial keeps on calling Dial until it stops returning an error.
	Stop()          // Stop shuts down the given AutoRedialer, if it is still retrying.
}

type redialerState uint32

const (
	Unconfigured redialerState = iota
	Redialing
	Stopped
)

func (s *redialerState) String() string {
	return [3]string{"Unconfigured", "Redialing", "Stopped"}[uint32(*s)]
}

type autoRedialer struct {
	stateLock     sync.RWMutex
	stateValue    redialerState
	stopping      chan struct{}
	reallyStopped chan struct{}
	dial          func() error
	jitter        func(time.Duration) time.Duration
}

func (ar *autoRedialer) state() redialerState {
	ar.stateLock.RLock()
	defer ar.stateLock.RUnlock()
	return ar.stateValue
}

func (ar *autoRedialer) setState(s redialerState) {
	ar.stateLock.Lock()
	defer ar.stateLock.Unlock()
	ar.stateValue = s
}

func (ar *autoRedialer) setStateIfEqual(oldState, newState redialerState) bool {
	ar.stateLock.Lock()
	defer ar.stateLock.Unlock()
	if ar.stateValue != oldState {
		return false
	}
	ar.stateValue = newState
	return true
}

func (ar *autoRedialer) setStateStopped() {
	ar.stateLock.Lock()
	defer ar.stateLock.Unlock()
	switch ar.stateValue {
	case Stopped:
		return
	case Unconfigured:
		close(ar.reallyStopped)
	}
	ar.stateValue = Stopped
	close(ar.stopping)
}

func (ar *autoRedialer) Stop() {
	if ar != nil {
		ar.setStateStopped()
		<-ar.reallyStopped
	}
}

// Redial keeps on calling Dial until it stops returning an error.  It does
// exponential backoff, adding back the output of Jitter at each step.
func (ar *autoRedialer) Redial() uint32 {
	if ar == nil {
		// at least it's better than a segfault...
		panic("you can't Redial a nil AutoRedialer")
	}
	if !ar.setStateIfEqual(Unconfigured, Redialing) {
		// XXX log this
		return 0
	}
	defer close(ar.reallyStopped)

	var timeout time.Duration
	var dialAttempts uint32 = 0 // unsigned so it can wrap safely ...
	timeouts := Timeouts()
	var numTimeouts uint32 = uint32(len(timeouts))
	for {
		if ar.state() != Redialing {
			return dialAttempts
		}
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
		case <-ar.stopping:
		case <-time.After(timeout):
		}
	}
}

// Returns a stoppable AutoRedialer using the provided Dialer. If the Dialer
// is also a Jitterer, the backoff will be jittered.
func NewAutoRedialer(dialer Dialer) AutoRedialer {
	ar := &autoRedialer{
		stateValue:    Unconfigured,
		dial:          dialer.Dial,
		reallyStopped: make(chan struct{}),
		stopping:      make(chan struct{}),
	}
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
