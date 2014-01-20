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

// bus/connection/testing provides an implementation of
// connection.Interface that takes a condition.Interface to determine
// whether to work.
package testing

import (
	"errors"
	"launchpad.net/ubuntu-push/bus/connection"
	"launchpad.net/ubuntu-push/testing/condition"
	"time"
)

type TestingConnection struct {
	cond    condition.Interface
	retvals []interface{}
}

// Build a TestingConnection that calls OK() on its condition before returning
// the provided return values.
//
// NOTE: Call() always returns the first return value; Watch() will provide
// each of them intern, irrespective of whether Call has been called.
func New(cond condition.Interface, retvals ...interface{}) *TestingConnection {
	return &TestingConnection{cond, retvals}
}

// See connection.WatchSignal. This WatchSignal will check its condition to decide
// whether to return an error, or provide each of the given return values
func (tc *TestingConnection) WatchSignal(member string, f func(interface{}), d func()) error {
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

// See connection.Call. This Call will check its condition to decide whether to
// return an error, or the first return value provided to New()
func (tc *TestingConnection) Call(member string, args ...interface{}) (interface{}, error) {
	if tc.cond.OK() {
		if len(tc.retvals) == 0 {
			panic("No return values provided!")
		}
		return tc.retvals[0], nil
	} else {
		return 0, errors.New("no way")
	}
}

// see connection.Close
func (tc *TestingConnection) Close() {}

// ensure TestingConnection implements connection.Interface
var _ connection.Interface = &TestingConnection{}
