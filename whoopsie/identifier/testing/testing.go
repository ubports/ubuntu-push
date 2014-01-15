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

// Package whoopsie/identifier/testing implements a couple of Ids that
// are useful for testing things that use whoopsie/identifier.
package testing

import "errors"

// SettableIdentifier is an Id that lets you set the value of the identifier.
//
// By default the identifier's value is "<Settable>", so it's visible
// if you're misusing it.
type SettableIdentifier struct {
	value string
}

// Settable is the constructor for SettableIdentifier.
func Settable() *SettableIdentifier {
	return &SettableIdentifier{"<Settable>"}
}

// Set is the method you use to set the identifier.
func (sid *SettableIdentifier) Set(value string) {
	sid.value = value
}

// Generate does nothing.
func (sid *SettableIdentifier) Generate() error {
	return nil
}

// String returns the string you set.
func (sid *SettableIdentifier) String() string {
	return sid.value
}

// FailingIdentifier is an Id that always fails to generate.
type FailingIdentifier struct{}

// Failing is the constructor for FailingIdentifier.
func Failing() *FailingIdentifier {
	return &FailingIdentifier{}
}

// Generate fails with an ubiquitous error.
func (*FailingIdentifier) Generate() error {
	return errors.New("lp0 on fire")
}

// String returns "<Failing>".
//
// The purpose of this is to make it easy to spot if you're using it
// by accident.
func (*FailingIdentifier) String() string {
	return "<Failing>"
}
