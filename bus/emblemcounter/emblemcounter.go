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
	"sync"

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
	bus  bus.Endpoint
	log  logger.Logger
	tags map[string]string
	lock sync.RWMutex
}

// Build an EmblemCounter using the given bus and log.
func New(endp bus.Endpoint, log logger.Logger) *EmblemCounter {
	return &EmblemCounter{bus: endp, log: log, tags: make(map[string]string)}
}

// Tags returns the notification tags for the given app
func (ctr *EmblemCounter) Tags(app *click.AppId) []string {
	tag, ok := ctr.tags[app.Original()]
	if !ok {
		return nil
	}
	return []string{tag}
}

func (ctr *EmblemCounter) tag(orig string) (string, bool) {
	ctr.lock.RLock()
	defer ctr.lock.RUnlock()

	tag, ok := ctr.tags[orig]
	return tag, ok
}

// Look for an EmblemCounter section in a Notification and, if
// present, presents it to the user.
func (ctr *EmblemCounter) Present(app *click.AppId, nid string, notification *launch_helper.Notification) bool {
	if notification == nil {
		panic("please check notification is not nil before calling present")
	}

	ec := notification.EmblemCounter

	if ec == nil {
		ctr.log.Debugf("[%s] notification has no EmblemCounter: %#v", nid, ec)
		return false
	}
	ctr.log.Debugf("[%s] setting emblem counter for %s to %d (visible: %t)", nid, app.Base(), ec.Count, ec.Visible)
	return ctr.present(app, notification.Tag, ec.Count, ec.Visible)
}

func (ctr *EmblemCounter) present(app *click.AppId, tag string, count int32, visible bool) bool {
	base := app.Base()
	quoted := string(nih.Quote([]byte(base)))

	err := ctr.bus.SetProperty("count", "/"+quoted, dbus.Variant{count})
	if err != nil {
		ctr.log.Errorf("call to set count failed: %v", err)
		return false
	}
	err = ctr.bus.SetProperty("countVisible", "/"+quoted, dbus.Variant{visible})
	if err != nil {
		ctr.log.Errorf("call to set countVisible failed: %v", err)
		return false
	}

	ctr.lock.Lock()
	defer ctr.lock.Unlock()
	if visible && count != 0 {
		ctr.tags[app.Original()] = tag
	} else {
		delete(ctr.tags, app.Original())
	}

	return true
}
