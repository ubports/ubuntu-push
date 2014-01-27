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
	"time"
)

// here we implement Retrier, which is a thing that takes a Dialer
// and retries its Dial method until it stops returning an error

type Dialer interface {
	Dial() error
	String() string
	Jitter() time.Duration
}

var Timeouts []time.Duration

var ( //  for use in testing
	quitRedialing chan bool = make(chan bool)
)

// Jitter returns a random time.Duration somewhere in [-spread, spread].
//
// This is meant as a default implementation for Dialers to use if wanted.
func Jitter(spread int) time.Duration {
	return time.Duration(rand.Intn(2*spread+1)-spread) * time.Second
}

// AutoRedial keeps on calling Dial() on the given Dialer until it stops
// returning an error.
func AutoRedial(dialer Dialer) uint32 {
	var timeout time.Duration
	var dialAttempts uint32 = 0 // unsigned so it can wrap safely ...
	var numTimeouts uint32 = uint32(len(Timeouts))
	for {
		if dialer.Dial() == nil {
			return dialAttempts + 1
		}
		if dialAttempts < numTimeouts {
			timeout = Timeouts[dialAttempts]
		} else {
			timeout = Timeouts[numTimeouts-1] + dialer.Jitter()
		}
		dialAttempts++
		select {
		case <-quitRedialing:
			return dialAttempts
		case <-time.NewTimer(timeout).C:
		}
	}
}

func init() {
	ps := []int{1, 2, 5, 11, 19, 37, 67, 113, 191} // 3 pₙ₊₁ ≥ 5 pₙ
	Timeouts = make([]time.Duration, len(ps))
	for i, n := range ps {
		Timeouts[i] = time.Duration(n) * time.Second
	}

	rand.Seed(time.Now().Unix()) // good enough for us (not crypto, yadda)
}
