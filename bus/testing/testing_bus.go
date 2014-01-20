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

// Package bus/testing provides an implementation of bus.Bus and bus.Endpoint
// suitable for testing.
package testing

// Here, the bus.Bus implementation.

import (
	"errors"
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/testing/condition"
)

/*****************************************************************
 *    TestingBus
 */

type testingBus struct {
	TestCond condition.Interface
	TestEndp *testingEndpoint
}

// Build a bus.Bus that takes a condition to determine whether it should work,
// as well as a condition and series of return values for the testing
// bus.Endpoint it builds.
func NewTestingBus(clientTC condition.Interface, busTC condition.Interface, retvals ...interface{}) *testingBus {
	return &testingBus{clientTC, NewTestingEndpoint(busTC, retvals...)}
}

// ensure testingBus implements bus.Interface
var _ bus.Bus = &testingBus{}

/*
   public methods
*/

func (tb *testingBus) Connect(info bus.Address, log logger.Logger) (bus.Endpoint, error) {
	if tb.TestCond.OK() {
		return tb.TestEndp, nil
	} else {
		return nil, errors.New(tb.TestCond.String())
	}
}

func (tb *testingBus) String() string {
	return "<TestingBus>"
}