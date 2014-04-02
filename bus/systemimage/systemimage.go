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

// Package systemimage is a mimimal wrapper for the system-image dbus API.
package systemimage

import (
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/logger"
)

// system-image service lives on a well-known bus.Address
var BusAddress bus.Address = bus.Address{
	Interface: "com.canonical.SystemImage",
	Path:      "/Service",
	Name:      "com.canonical.SystemImage",
}

// InfoResult holds the result of the system-image service Info method.
type InfoResult struct {
	BuildNumber int32
	Device      string
	Channel     string
	// xxx channel_target missing
	LastUpdate    string
	VersionDetail map[string]string
}

// A SystemImage exposes the a subset of system-image service.
type SystemImage interface {
	Info() (*InfoResult, error)
}

type systemImage struct {
	endp bus.Endpoint
	log  logger.Logger
}

// New builds a new system-image service wrapper that uses the provided bus.Endpoint
func New(endp bus.Endpoint, log logger.Logger) SystemImage {
	return &systemImage{endp, log}
}

var _ SystemImage = &systemImage{} // ensures it conforms

func (si *systemImage) Info() (*InfoResult, error) {
	si.log.Debugf("Invoking Info")
	res := &InfoResult{}
	err := si.endp.Call("Info", bus.Args(), &res.BuildNumber, &res.Device, &res.Channel, &res.LastUpdate, &res.VersionDetail)
	if err != nil {
		si.log.Errorf("Info failed: %v", err)
		return nil, err
	}
	return res, err
}
