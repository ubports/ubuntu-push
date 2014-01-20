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

// Package bus/testing provides an implementation of bus.Interface
// that takes condition.Interface to determine whether to work, as
// well using a bus connection from connection/testing.
package testing

import (
	"errors"
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/bus/connection"
	"launchpad.net/ubuntu-push/bus/connection/testing"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/testing/condition"
)

type TestingBus struct {
	TestCond condition.Interface
	TestConn *testing.TestingConnection
}

func (tb *TestingBus) Connect(info bus.Info, log logger.Logger) (connection.Interface, error) {
	if tb.TestCond.OK() {
		return tb.TestConn, nil
	} else {
		return nil, errors.New(tb.TestCond.String())
	}
}

func (tb *TestingBus) String() string {
	return "<TestingBus>"
}

// New takes a condition to determine whether it should work, as well as a
// condition and series of return values for the testing.TestingConnection it
// builds.
func New(clientTC condition.Interface, busTC condition.Interface, retvals ...interface{}) *TestingBus {
	return &TestingBus{clientTC, testing.New(busTC, retvals...)}
}

var _ bus.Interface = &TestingBus{}
