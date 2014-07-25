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

// Package urldispatcher wraps the url dispatcher's dbus api point
package urldispatcher

import (
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/logger"
)

// UrlDispatcher lives on a well-known bus.Address
var BusAddress bus.Address = bus.Address{
	Interface: "com.canonical.URLDispatcher",
	Path:      "/com/canonical/URLDispatcher",
	Name:      "com.canonical.URLDispatcher",
}

// A URLDispatcher is a simple beast, with a single method that does what it
// says on the box.
type URLDispatcher interface {
	DispatchURL(string, *click.AppId) error
	TestURL(*click.AppId, []string) bool
}

type urlDispatcher struct {
	endp bus.Endpoint
	log  logger.Logger
}

// New builds a new URL dispatcher that uses the provided bus.Endpoint
func New(endp bus.Endpoint, log logger.Logger) URLDispatcher {
	return &urlDispatcher{endp, log}
}

var _ URLDispatcher = &urlDispatcher{} // ensures it conforms

func (ud *urlDispatcher) DispatchURL(url string, app *click.AppId) error {
	ud.log.Debugf("Dispatching %s", url)
	err := ud.endp.Call("DispatchURL", bus.Args(url, app.Base()))
	if err != nil {
		ud.log.Errorf("Dispatch to %s failed with %s", url, err)
	}
	return err
}

func (ud *urlDispatcher) TestURL(app *click.AppId, urls []string) bool {
	ud.log.Debugf("TestURL: %s", urls)
	var appIds []string
	err := ud.endp.Call("TestURL", bus.Args(urls), &appIds)
	if err != nil {
		ud.log.Errorf("TestURL for %s failed with %s", urls, err)
		return false
	}
	for _, appId := range appIds {
		if appId != app.Versioned() {
			ud.log.Debugf("Notification skipped because of different appid for actions: %v - %s != %s", urls, appId, app.Versioned())
			return false
		}
	}
	return true
}
