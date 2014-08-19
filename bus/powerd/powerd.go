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

// Package powerd is an interface to powerd via dbus.
package powerd

import (
	"errors"
	"time"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/logger"
)

// powerd lives on a well-known bus.Address
var BusAddress bus.Address = bus.Address{
	Interface: "com.canonical.powerd",
	Path:      "/com/canonical/powerd",
	Name:      "com.canonical.powerd",
}

type Wakeup struct{}

// Powerd exposes a subset of powerd
type Powerd interface {
	RequestWakeup(name string, wakeupTime time.Time) (string, error)
	ClearWakeup(cookie string) error
	WatchWakeups() (<-chan *Wakeup, error)
	RequestWakelock(name string) (string, error)
	ClearWakelock(cookie string) error
}

type powerd struct {
	endp bus.Endpoint
	log  logger.Logger
}

var (
	ErrUnconfigured = errors.New("unconfigured.")
)

// New builds a new Powerd that uses the provided bus.Endpoint
func New(endp bus.Endpoint, log logger.Logger) Powerd {
	return &powerd{endp, log}
}

func (p *powerd) RequestWakeup(name string, wakeupTime time.Time) (string, error) {
	if p.endp == nil {
		return "", ErrUnconfigured
	}
	var res string
	err := p.endp.Call("requestWakeup", bus.Args(name, wakeupTime.Unix()), &res)
	return res, err
}

func (p *powerd) ClearWakeup(cookie string) error {
	if p.endp == nil {
		return ErrUnconfigured
	}
	return p.endp.Call("clearWakeup", bus.Args(cookie))
}

func (p *powerd) WatchWakeups() (<-chan *Wakeup, error) {
	if p.endp == nil {
		return nil, ErrUnconfigured
	}
	ch := make(chan *Wakeup)
	p.endp.WatchSignal("Wakeup", func(...interface{}) {
		ch <- &Wakeup{}
	}, func() { close(ch) })
	return ch, nil
}

func (p *powerd) RequestWakelock(name string) (string, error) {
	// wakelocks are documented on https://wiki.ubuntu.com/powerd#API
	// (requestSysState with state=1)
	if p.endp == nil {
		return "", ErrUnconfigured
	}
	var res string
	err := p.endp.Call("requestSysState", bus.Args(name, int32(1)), &res)
	return res, err
}

func (p *powerd) ClearWakelock(cookie string) error {
	if p.endp == nil {
		return ErrUnconfigured
	}
	return p.endp.Call("clearSysState", bus.Args(cookie))
}
