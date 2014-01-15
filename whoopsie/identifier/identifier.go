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

// Package whoopsie/identifier wraps libwhoopsie, and is thus the
// source of an anonymous and stable system id used by the Ubuntu
// error tracker and the Ubuntu push notifications service.
package identifier

/*
#cgo pkg-config: libwhoopsie
#include <glib.h>
#include <libwhoopsie/identifier.h>
*/
import "C"
import "unsafe"
import "errors"

// an Id knows how to generate itself, and how to stringify itself.
type Id interface {
	Generate() error
	String() string
}

// Identifier is the default Id implementation.
type Identifier struct {
	value string
}

// New creates an Identifier, but does not call Generate() on it.
func New() *Identifier {
	return &Identifier{}
}

// Generate makes the Identifier create the identifier itself.
func (id *Identifier) Generate() error {
	var gerr *C.GError
	var cs *C.char
	defer C.g_free((C.gpointer)(unsafe.Pointer(cs)))
	C.whoopsie_identifier_generate(&cs, &gerr)

	if gerr != nil {
		return errors.New(C.GoString((*C.char)(gerr.message)))
	} else {
		id.value = C.GoString(cs)
		return nil
	}

}

// String returns the system identifier as a string.
func (id *Identifier) String() string {
	return id.value
}
