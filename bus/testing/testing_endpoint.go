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
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/testing/condition"
	"time"
)

type testingEndpoint struct {
	cond    condition.Interface
	retvals []interface{}
}

// Build a bus.Endpoint that calls OK() on its condition before returning
// the provided return values.
//
// NOTE: Call() always returns the first return value; Watch() will provide
// each of them intern, irrespective of whether Call has been called.
func NewTestingEndpoint(cond condition.Interface, retvals ...interface{}) bus.Endpoint {
	return &testingEndpoint{cond, retvals}
}

// See Endpoint's WatchSignal. This WatchSignal will check its condition to
// decide whether to return an error, or provide each of its return values
func (tc *testingEndpoint) WatchSignal(member string, f func(interface{}), d func()) error {
	if tc.cond.OK() {
		go func() {
			for _, v := range tc.retvals {
				f(v)
				time.Sleep(10 * time.Millisecond)
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
func (tc *testingEndpoint) Call(member string, args ...interface{}) (interface{}, error) {
	if tc.cond.OK() {
		if len(tc.retvals) == 0 {
			panic("No return values provided!")
		}
		return tc.retvals[0], nil
	} else {
		return 0, errors.New("no way")
	}
}

// See Endpoint's GetProperty. This one is just another name for Call.
func (tc *testingEndpoint) GetProperty(property string) (interface{}, error) {
	return tc.Call(property)
}

// see Endpoint's Close. This one does nothing.
func (tc *testingEndpoint) Close() {}

// ensure testingEndpoint implements bus.Endpoint
var _ bus.Endpoint = &testingEndpoint{}
