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

// Package testing provides an implementation of bus.Bus and bus.Endpoint
// suitable for testing.
package testing

// Here, the bus.Bus implementation.

import (
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/testing/condition"
)

/*****************************************************************
 *    TestingBus
 */

type testingBus struct {
	endp bus.Endpoint
}

// Build a bus.Bus that takes a condition to determine whether it should work,
// as well as a condition and series of return values for the testing
// bus.Endpoint it builds.
func NewTestingBus(dialTC condition.Interface, callTC condition.Interface, retvals ...interface{}) bus.Bus {
	return &testingBus{NewTestingEndpoint(dialTC, callTC, retvals...)}
}

// Build a bus.Bus that takes a condition to determine whether it should work,
// as well as a condition and a series of lists of return values for the
// testing bus.Endpoint it builds.
func NewMultiValuedTestingBus(dialTC condition.Interface, callTC condition.Interface, retvalses ...[]interface{}) bus.Bus {
	return &testingBus{NewMultiValuedTestingEndpoint(dialTC, callTC, retvalses...)}
}

// ensure testingBus implements bus.Interface
var _ bus.Bus = &testingBus{}

/*
   public methods
*/

func (tb *testingBus) Endpoint(info bus.Address, log logger.Logger) bus.Endpoint {
	return tb.endp
}

func (tb *testingBus) String() string {
	return "<TestingBus>"
}
