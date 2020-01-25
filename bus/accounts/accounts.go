/*
 Copyright 2013-2015 Canonical Ltd.

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
// accounts exposes some properties that're stored in org.freedesktop.Accounts
// (specifically, the ones that we need are all under
// com.ubuntu.touch.AccountsService.Sound).
package accounts

import (
	"fmt"
	"os/user"
	"strings"
	"sync"

	"launchpad.net/go-dbus"
	"launchpad.net/go-xdg"

	"github.com/ubports/ubuntu-push/bus"
	"github.com/ubports/ubuntu-push/logger"
)

// accounts lives on a well-known bus.Address.
//
// Note this one isn't it: the interface is for dbus.properties, and the path
// is missing the UID.
var BusAddress bus.Address = bus.Address{
	Interface: "org.freedesktop.DBus.Properties",
	Path:      "/org/freedesktop/Accounts/User",
	Name:      "org.freedesktop.Accounts",
}

const accountsSoundIface = "com.ubuntu.touch.AccountsService.Sound"

type Accounts interface {
	// Start() sets up the asynchronous updating of properties, and does the first update.
	Start() error
	// Cancel() stops the asynchronous updating of properties.
	Cancel() error
	// SilentMode() tells you whether the device is in silent mode.
	SilentMode() bool
	// Vibrate() tells you whether the device is allowed to vibrate.
	Vibrate() bool
	// MessageSoundFile() tells you the default sound filename.
	MessageSoundFile() string
	String() string
}

// Accounts tracks the relevant bits of configuration. Nothing directly
// accessible because it is updated asynchronously, so use the accessors.
type accounts struct {
	endp              bus.Endpoint
	log               logger.Logger
	silent            bool
	vibrate           bool
	vibrateSilentMode bool
	messageSound      string
	cancellable       bus.Cancellable
	lck               sync.Mutex
	updaters          map[string]func(dbus.Variant)
}

// sets up a new Accounts structure, ready to be Start()ed.
func New(endp bus.Endpoint, log logger.Logger) Accounts {
	a := &accounts{
		endp: endp,
		log:  log,
	}

	a.updaters = map[string]func(dbus.Variant){
		"SilentMode":                       a.updateSilentMode,
		"IncomingMessageVibrate":           a.updateVibrate,
		"IncomingMessageVibrateSilentMode": a.updateVibrateSilentMode,
		"IncomingMessageSound":             a.updateMessageSound,
	}

	return a
}

// sets up the asynchronous updating of properties, and does the first update.
func (a *accounts) Start() error {
	err := a.startWatch()
	if err != nil {
		return err
	}
	a.update()
	return nil
}

// does sets up the watch on the PropertiesChanged signal. Separate from Start
// because it holds a lock.
func (a *accounts) startWatch() error {
	cancellable, err := a.endp.WatchSignal("PropertiesChanged", a.propsHandler, a.bailoutHandler)
	if err != nil {
		a.log.Errorf("unable to watch for property changes: %v", err)
		return err
	}

	a.lck.Lock()
	defer a.lck.Unlock()
	if a.cancellable != nil {
		panic("tried to start Accounts twice?")
	}
	a.cancellable = cancellable

	return nil
}

// cancel the asynchronous updating of properties.
func (a *accounts) Cancel() error {
	return a.cancellable.Cancel()
}

// slightly shorter than %#v
func (a *accounts) String() string {
	return fmt.Sprintf("&accounts{silent: %t, vibrate: %t, vibratesilent: %t, messageSound: %q}",
		a.silent, a.vibrate, a.vibrateSilentMode, a.messageSound)
}

// merely log that the watch loop has bailed; not much we can do.
func (a *accounts) bailoutHandler() {
	a.log.Debugf("loop bailed out")
}

// handle PropertiesChanged, which is described in
// http://dbus.freedesktop.org/doc/dbus-specification.html#standard-interfaces-properties
func (a *accounts) propsHandler(ns ...interface{}) {
	if len(ns) != 3 {
		a.log.Errorf("PropertiesChanged delivered %d things instead of 3.", len(ns))
		return
	}

	iface, ok := ns[0].(string)
	if !ok {
		a.log.Errorf("PropertiesChanged 1st param not a string: %#v.", ns[0])
		return
	}
	if iface != accountsSoundIface {
		a.log.Debugf("PropertiesChanged for %#v, ignoring.", iface)
		return
	}
	changed, ok := ns[1].(map[interface{}]interface{})
	if !ok {
		a.log.Errorf("PropertiesChanged 2nd param not a map: %#v.", ns[1])
		return
	}
	if len(changed) != 0 {
		// not seen in the wild, but easy to implement properly (ie
		// using the values we're given) if it starts to
		// happen. Meanwhile just do a full update.
		a.log.Infof("PropertiesChanged provided 'changed'; reverting to full update.")
		a.update()
		return
	}
	invalid, ok := ns[2].([]interface{})
	if !ok {
		a.log.Errorf("PropertiesChanged 3rd param not a list of properties: %#v.", ns[2])
		return
	}
	a.log.Debugf("props changed: %#v.", invalid)
	switch len(invalid) {
	case 0:
		// nothing to do?
		a.log.Debugf("PropertiesChanged 3rd param is empty; doing nothing.")
	case 1:
		// the common case right now
		k, ok := invalid[0].(string)
		if !ok {
			a.log.Errorf("PropertiesChanged 3rd param's only entry not a string: %#v.", invalid[0])
			return
		}
		updater, ok := a.updaters[k]
		if ok {
			var v dbus.Variant
			err := a.endp.Call("Get", []interface{}{accountsSoundIface, k}, &v)
			if err != nil {
				a.log.Errorf("when calling Get for %s: %v", k, err)
				return
			}
			a.log.Debugf("Get for %s got %#v.", k, v)
			// updaters must be called with the lock held
			a.lck.Lock()
			defer a.lck.Unlock()
			updater(v)
			a.log.Debugf("updated %s.", k)
		}
	default:
		// not seen in the wild, but we probably want to drop to a
		// full update if getting more than one change anyway.
		a.log.Infof("PropertiesChanged provided more than one 'invalid'; reverting to full update.")
		a.update()
	}
}

func (a *accounts) updateSilentMode(vsilent dbus.Variant) {
	silent, ok := vsilent.Value.(bool)
	if !ok {
		a.log.Errorf("SilentMode needed a bool.")
		return
	}

	a.silent = silent
}

func (a *accounts) updateVibrate(vvibrate dbus.Variant) {
	vibrate, ok := vvibrate.Value.(bool)
	if !ok {
		a.log.Errorf("IncomingMessageVibrate needed a bool.")
		return
	}

	a.vibrate = vibrate
}

func (a *accounts) updateVibrateSilentMode(vvibrateSilentMode dbus.Variant) {
	vibrateSilentMode, ok := vvibrateSilentMode.Value.(bool)
	if !ok {
		a.log.Errorf("IncomingMessageVibrateSilentMode needed a bool.")
		return
	}

	a.vibrateSilentMode = vibrateSilentMode
}

func (a *accounts) updateMessageSound(vsnd dbus.Variant) {
	snd, ok := vsnd.Value.(string)
	if !ok {
		a.log.Errorf("IncomingMessageSound needed a string.")
		return
	}

	for _, dir := range xdg.Data.Dirs()[1:] {
		if dir[len(dir)-1] != '/' {
			dir += "/"
		}
		if strings.HasPrefix(snd, dir) {
			snd = snd[len(dir):]
			break
		}
	}

	a.messageSound = snd
}

func (a *accounts) update() {
	props := make(map[string]dbus.Variant)
	err := a.endp.Call("GetAll", []interface{}{accountsSoundIface}, &props)
	if err != nil {
		a.log.Errorf("when calling GetAll: %v", err)
		return
	}
	a.log.Debugf("GetAll got: %#v", props)

	a.lck.Lock()
	defer a.lck.Unlock()

	for name, updater := range a.updaters {
		updater(props[name])
	}
}

// is the device in silent mode?
func (a *accounts) SilentMode() bool {
	a.lck.Lock()
	defer a.lck.Unlock()

	return a.silent
}

// should notifications vibrate?
func (a *accounts) Vibrate() bool {
	a.lck.Lock()
	defer a.lck.Unlock()

	if a.silent {
		return a.vibrateSilentMode
	} else {
		return a.vibrate
	}
}

// what is the default sound file?
func (a *accounts) MessageSoundFile() string {
	a.lck.Lock()
	defer a.lck.Unlock()

	return a.messageSound
}

// the BusAddress should actually end with the UID of the user in question;
// here we do what's needed to get that.
func init() {
	u, err := user.Current()
	if err != nil {
		panic(err)
	}

	BusAddress.Path += u.Uid
}
