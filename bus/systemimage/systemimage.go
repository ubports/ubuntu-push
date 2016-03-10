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
	"strconv"
	"strings"

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
	Raw           map[string]string
}

// A SystemImage exposes the a subset of system-image service.
type SystemImage interface {
	Information() (*InfoResult, error)
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

func (si *systemImage) Information() (*InfoResult, error) {
	si.log.Debugf("invoking Information")
	m := map[string]string{}
	err := si.endp.Call("Information", bus.Args(), &m)

	if err != nil {
		si.log.Errorf("Information failed: %v", err)
		return nil, err
	}

	res := &InfoResult{}

	// Try parsing the build number if it exist.
	if bn := m["current_build_number"]; len(bn) > 0 {
		bn, err := strconv.ParseInt(bn, 10, 32)
		if err == nil {
			res.BuildNumber = int32(bn)
		} else {
			res.BuildNumber = -1
		}
	}

	res.Device = m["device_name"]
	res.Channel = m["channel_name"]
	res.LastUpdate = m["last_update_date"]
	res.VersionDetail = map[string]string{}

	// Split version detail key=value,key2=value2 into a string map
	// Note that even if
	vals := strings.Split(m["version_detail"], ",")
	for _, val := range vals {
		pairs := strings.Split(val, "=")
		if len(pairs) != 2 {
			continue
		}
		res.VersionDetail[pairs[0]] = pairs[1]
	}
	res.Raw = m

	return res, err
}
