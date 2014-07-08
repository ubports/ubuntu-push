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

// Package emblemcounter can present notifications as a counter on an
// emblem on an item in the launcher.
package emblemcounter

import (
	"launchpad.net/go-dbus/v1"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/nih"
)

// emblemcounter works by setting properties on a well-known dbus name.
var BusAddress = bus.Address{
	Interface: "com.canonical.Unity.Launcher.Item",
	Path:      "/com/canonical/Unity/Launcher",
	Name:      "com.canonical.Unity.Launcher",
}

// EmblemCounter is a little tool that fiddles with the unity launcher
// to put emblems with counters on launcher icons.
type EmblemCounter struct {
	bus bus.Endpoint
	log logger.Logger
}

// Build an EmblemCounter using the given bus and log.
func New(endp bus.Endpoint, log logger.Logger) *EmblemCounter {
	return &EmblemCounter{bus: endp, log: log}
}

// Look for an EmblemCounter section in a Notification and, if
// present, presents it to the user.
func (ctr *EmblemCounter) Present(appId *click.AppId, nid string, notification *launch_helper.Notification) {
	if notification == nil || notification.EmblemCounter == nil {
		ctr.log.Debugf("[%s] no notification or no EmblemCounter in the notification; doing nothing: %#v", nid, notification)
		return
	}
	ec := notification.EmblemCounter
	ctr.log.Debugf("[%s] setting emblem counter for %s to %d (visible: %t)", nid, appId.Application, ec.Count, ec.Visible)

	quoted := string(nih.Quote([]byte(appId.Application)))

	ctr.bus.SetProperty("count", "/"+quoted, dbus.Variant{ec.Count})
	ctr.bus.SetProperty("countVisible", "/"+quoted, dbus.Variant{ec.Visible})
}
