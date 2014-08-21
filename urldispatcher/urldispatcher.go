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

// Package urldispatcher wraps the url dispatcher's C API
package urldispatcher

import (
	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/urldispatcher/curldispatcher"
)

// A URLDispatcher is a simple beast, with a single method that does what it
// says on the box.
type URLDispatcher interface {
	DispatchURL(string, *click.AppId) error
	TestURL(*click.AppId, []string) bool
}

type urlDispatcher struct {
	log logger.Logger
}

// New builds a new URL dispatcher that uses the provided bus.Endpoint
func New(log logger.Logger) URLDispatcher {
	return &urlDispatcher{log}
}

var _ URLDispatcher = &urlDispatcher{} // ensures it conforms

var cDispatchURL = curldispatcher.DispatchURL
var cTestURL = curldispatcher.TestURL

func (ud *urlDispatcher) DispatchURL(url string, app *click.AppId) error {
	ud.log.Debugf("Dispatching %s", url)
	err := cDispatchURL(url, app.DispatchPackage())
	if err != nil {
		ud.log.Errorf("Dispatch to %s failed with %s", url, err)
	}
	return err
}

func (ud *urlDispatcher) TestURL(app *click.AppId, urls []string) bool {
	ud.log.Debugf("TestURL: %s", urls)
	var appIds []string
	appIds = cTestURL(urls)
	if len(appIds) == 0 {
		ud.log.Debugf("TestURL: invalid urls: %s - %s", urls, app.Versioned())
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
