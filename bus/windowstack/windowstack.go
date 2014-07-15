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

// Package windowstack retrieves information about the windowstack
// using Unity's dbus interface
package windowstack

import (
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/logger"
)

// import "fmt"

// Well known address for the WindowStack API
var BusAddress bus.Address = bus.Address{
	Interface: "com.canonical.Unity.WindowStack",
	Path:      "/com/canonical/Unity/WindowStack",
	Name:      "com.canonical.Unity.WindowStack",
}

type WindowsInfo struct {
	WindowId uint32
	AppId    string // in the form "com.ubuntu.calendar_calendar" or "webbrowser-app"
	Focused  bool
	Stage    uint32
}

// WindowStack encapsulates info needed to call out to the WindowStack API
type WindowStack struct {
	bus bus.Endpoint
	log logger.Logger
}

// New returns a new WindowStack that'll use the provided bus.Endpoint
func New(endp bus.Endpoint, log logger.Logger) *WindowStack {
	return &WindowStack{endp, log}
}

// GetWindowStack returns the window stack state
func (stack *WindowStack) GetWindowStack() []WindowsInfo {
	var wstack []WindowsInfo
	err := stack.bus.Call("GetWindowStack", bus.Args(), &wstack)
	if err != nil {
		stack.log.Debugf("GetWindowStack call returned %v", err)
	}
	return wstack
}

func (stack *WindowStack) IsAppFocused(AppId *click.AppId) bool {
	for _, winfo := range stack.GetWindowStack() {
		if winfo.Focused && winfo.AppId == AppId.Package+"_"+AppId.Application {
			return true
		}
	}
	return false
}
