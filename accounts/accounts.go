/*
 Copyright 2014 Canonical Ltd.

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

// Package accounts wraps libaccounts
package accounts

/*
#cgo pkg-config: glib-2.0 libaccounts-glib
#include <glib.h>

void start();

*/
import "C"

type Changed struct{}

var ch chan Changed

//export gocb
func gocb() {
	ch <- Changed{}
}

func Watch() <-chan Changed {
	ch = make(chan Changed, 1)
	C.start()

	return ch
}
