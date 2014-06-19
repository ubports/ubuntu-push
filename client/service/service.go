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

// package service implements the dbus-level service with which client
// applications are expected to interact.
package service

import (
	"os"
	"strings"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/nih"
)

// Service is the dbus api
type Service struct {
	DBusService
	regURL     string
	authGetter func(string) string
}

var (
	ServiceBusAddress = bus.Address{
		Interface: "com.ubuntu.PushNotifications",
		Path:      "/com/ubuntu/PushNotifications/*",
		Name:      "com.ubuntu.PushNotifications",
	}
)

// NewService() builds a new service and returns it.
func NewService(bus bus.Endpoint, log logger.Logger) *Service {
	var svc = &Service{}
	svc.Log = log
	svc.Bus = bus
	return svc
}

// SetRegistrationURL() sets the registration url for the service
func (svc *Service) SetRegistrationURL(url string) {
	svc.Lock.Lock()
	defer svc.Lock.Unlock()
	svc.regURL = url
}

// SetAuthGetter() sets the authorization getter for the service
func (svc *Service) SetAuthGetter(authGetter func(string) string) {
	svc.Lock.Lock()
	defer svc.Lock.Unlock()
	svc.authGetter = authGetter
}

// GetRegistrationAuthorization() returns the authorization header for
// POSTing to the registration HTTP endpoint
func (svc *Service) GetRegistrationAuthorization() string {
	svc.Lock.RLock()
	defer svc.Lock.RUnlock()
	if svc.authGetter != nil && svc.regURL != "" {
		return svc.authGetter(svc.regURL)
	} else {
		return ""
	}
}

// Start() dials the bus, grab the name, and listens for method calls.
func (svc *Service) Start() error {
	return svc.DBusService.Start(bus.DispatchMap{
		"Register": svc.register,
	}, ServiceBusAddress)
}

func (svc *Service) register(path string, args, _ []interface{}) ([]interface{}, error) {
	if len(args) != 0 {
		return nil, BadArgCount
	}
	raw_appname := path[strings.LastIndex(path, "/")+1:]
	appname := string(nih.Unquote([]byte(raw_appname)))

	rv := os.Getenv("PUSH_REG_" + raw_appname)
	if rv == "" {
		rv = appname + "::this-is-an-opaque-block-of-random-bits-i-promise"
	}

	return []interface{}{rv}, nil
}
