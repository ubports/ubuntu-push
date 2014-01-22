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

package main

import (
	"launchpad.net/ubuntu-push/logger"
	"net"
	"os"
)

// boot logging and hooks

func bootLogListener(kind string, lst net.Listener) {
	BootLogger.Infof("listening for %s on %v", kind, lst.Addr())
}

var (
	BootLogger      = logger.NewSimpleLogger(os.Stderr, "debug")
	BootLogListener = bootLogListener
	BootLogFatalf   = BootLogger.Fatalf
)
