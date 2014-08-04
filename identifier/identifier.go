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

// Package identifier is the source of an anonymous and stable
// system id (from /var/lib/dbus/machine-id) used by the Ubuntu
// push notifications service.
package identifier

import (
	"fmt"
	"io/ioutil"
)

var machineIdPath = "/var/lib/dbus/machine-id"

// an Id knows how to generate itself, and how to stringify itself.
type Id interface {
	String() string
}

// Identifier is the default Id implementation.
type Identifier struct {
	value string
}

func readMachineId() (string, error) {
	value, err := ioutil.ReadFile(machineIdPath)
	if err != nil {
		return "", err
	}
	return string(value)[:len(value)-1], nil
}

// New creates an Identifier
func New() (Id, error) {
	value, err := readMachineId()
	if err != nil {
		return &Identifier{value: ""}, fmt.Errorf("Failed to read the machine id: %s", err)
	}
	return &Identifier{value: value}, nil
}

// String returns the system identifier as a string.
func (id *Identifier) String() string {
	return id.value
}
