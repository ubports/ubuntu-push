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

// Package haptic can present notifications as a vibration pattern
// using the usensord/haptic interface
package haptic

import (
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/logger"
)

// usensord/haptic lives on a well-knwon bus.Address
var BusAddress bus.Address = bus.Address{
	Interface: "com.canonical.usensord.haptic",
	Path:      "/com/canonical/usensord/haptic",
	Name:      "com.canonical.usensord",
}

// Haptic encapsulates info needed to call out to usensord/haptic
type Haptic struct {
	bus bus.Endpoint
	log logger.Logger
}

// New returns a new Haptic that'll use the provided bus.Endpoint
func New(endp bus.Endpoint, log logger.Logger) *Haptic {
	return &Haptic{endp, log}
}

// Present presents the notification via a vibrate pattern
func (haptic *Haptic) Present(_, _ string, notification *launch_helper.Notification) bool {
	if notification == nil || notification.Vibrate == nil {
		haptic.log.Debugf("no notification or no Vibrate in the notification; doing nothing: %#v", notification)
		return false
	}
	pattern := notification.Vibrate.Pattern
	repeat := notification.Vibrate.Repeat
	if repeat == 0 {
		repeat = 1
	}
	if notification.Vibrate.Duration != 0 {
		pattern = []uint32{notification.Vibrate.Duration}
	}
	if len(pattern) == 0 {
		haptic.log.Debugf("not enough information in the notification's Vibrate section to create a pattern")
		return false
	}
	haptic.log.Debugf("vibrating %d times to the tune of %v", repeat, pattern)
	err := haptic.bus.Call("VibratePattern", bus.Args(pattern, repeat))
	if err != nil {
		haptic.log.Debugf("VibratePattern call returned %v", err)
		return false
	}
	return true
}
