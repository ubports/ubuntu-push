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

// Package polld wraps the account-polld dbus interface
package polld

import (
	"errors"

	"github.com/ubports/ubuntu-push/bus"
	"github.com/ubports/ubuntu-push/logger"
)

var (
	ErrUnconfigured = errors.New("unconfigured.")
)

// polld lives on a well-known bus.Address
var BusAddress bus.Address = bus.Address{
	Interface: "com.ubuntu.AccountPolld",
	Path:      "/com/ubuntu/AccountPolld",
	Name:      "com.ubuntu.AccountPolld",
}

type Polld interface {
	Poll() error
	WatchDones() (<-chan bool, error)
}

type polld struct {
	endp bus.Endpoint
	log  logger.Logger
}

func New(endp bus.Endpoint, log logger.Logger) Polld {
	return &polld{endp, log}
}

func (p *polld) Poll() error {
	if p.endp == nil {
		return ErrUnconfigured
	}
	return p.endp.Call("Poll", nil)
}

func (p *polld) WatchDones() (<-chan bool, error) {
	if p.endp == nil {
		return nil, ErrUnconfigured
	}
	ch := make(chan bool)
	p.endp.WatchSignal("Done", func(...interface{}) {
		ch <- true
	}, func() { close(ch) })
	return ch, nil
}
