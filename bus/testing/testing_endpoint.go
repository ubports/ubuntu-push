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

package testing

// Here, the bus.Endpoint implementation.

import (
	"errors"
	"fmt"
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/testing/condition"
	"time"
)

type testingEndpoint struct {
	dialCond condition.Interface
	callCond condition.Interface
	retvals  [][]interface{}
}

// Build a bus.Endpoint that calls OK() on its condition before returning
// the provided return values.
//
// NOTE: Call() always returns the first return value; Watch() will provide
// each of them in turn, irrespective of whether Call has been called.
func NewMultiValuedTestingEndpoint(dialCond condition.Interface, callCond condition.Interface, retvalses ...[]interface{}) bus.Endpoint {
	return &testingEndpoint{dialCond, callCond, retvalses}
}

func NewTestingEndpoint(dialCond condition.Interface, callCond condition.Interface, retvals ...interface{}) bus.Endpoint {
	retvalses := make([][]interface{}, len(retvals))
	for i, x := range retvals {
		retvalses[i] = []interface{}{x}
	}
	return &testingEndpoint{dialCond, callCond, retvalses}
}

// if WatchTickeris not nil, it is used instead of the default timeout
// to wait while sending values over WatchSignal
var WatchTicker chan bool

// See Endpoint's WatchSignal. This WatchSignal will check its condition to
// decide whether to return an error, or provide each of its return values
func (tc *testingEndpoint) WatchSignal(member string, f func(...interface{}), d func()) error {
	if tc.callCond.OK() {
		go func() {
			for _, v := range tc.retvals {
				f(v...)
				if WatchTicker != nil {
					<-WatchTicker
				} else {
					<-time.Tick(10 * time.Millisecond)
				}
			}
			d()
		}()
		return nil
	} else {
		return errors.New("no way")
	}
}

// See Endpoint's Call. This Call will check its condition to decide whether
// to return an error, or the first of its return values
func (tc *testingEndpoint) Call(member string, args ...interface{}) ([]interface{}, error) {
	if tc.callCond.OK() {
		if len(tc.retvals) == 0 {
			panic("No return values provided!")
		}
		return tc.retvals[0], nil
	} else {
		return nil, errors.New("no way")
	}
}

// See Endpoint's GetProperty. This one is just another name for Call.
func (tc *testingEndpoint) GetProperty(property string) (interface{}, error) {
	rvs, err := tc.Call(property)
	if err != nil {
		return nil, err
	}
	if len(rvs) != 1 {
		return nil, errors.New("Wrong number of values given to testingEndpoint" +
			" -- GetProperty only returns a single value for now!")
	}
	return rvs[0], err
}

// See Endpoint's Dial. This one will check its dialCondition to
// decide whether to return an error or not.
func (endp *testingEndpoint) Dial() error {
	if endp.dialCond.OK() {
		return nil
	} else {
		return errors.New("dialCond said No.")
	}
}

// Advanced stringifobabulation
func (endp *testingEndpoint) String() string {
	return fmt.Sprintf("&testingEndpoint{dialCond:(%s) callCond:(%s) retvals:(%#v)",
		endp.dialCond, endp.callCond, endp.retvals)
}

// see Endpoint's Close. This one does nothing.
func (tc *testingEndpoint) Close() {}

// see Endpoint's Jitter.
func (tc *testingEndpoint) Jitter(_ time.Duration) time.Duration { return 0 }

// ensure testingEndpoint implements bus.Endpoint
var _ bus.Endpoint = &testingEndpoint{}
