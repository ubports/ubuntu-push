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

// package service implements the dbus-level service with which client
// applications are expected to interact.
package service

import (
	"errors"
	"sync"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/logger"
)

type DBusService struct {
	lock  sync.RWMutex
	state ServiceState
	Log   logger.Logger
	Bus   bus.Endpoint
}

// the service can be in a numnber of states
type ServiceState uint8

const (
	StateUnknown  ServiceState = iota
	StateRunning               // Start() has been successfully called
	StateFinished              // Stop() has been successfully called
)

var (
	NotConfigured  = errors.New("not configured")
	AlreadyStarted = errors.New("already started")
	BadArgCount = errors.New("Wrong number of arguments")
	BadArgType  = errors.New("Bad argument type")
)

// IsRunning() returns whether the service's state is StateRunning
func (svc *DBusService) IsRunning() bool {
	svc.lock.RLock()
	defer svc.lock.RUnlock()
	return svc.state == StateRunning
}

// Start() dials the bus, grab the name, and listens for method calls.
func (svc *DBusService) Start(dispatchMap bus.DispatchMap, busAddr bus.Address) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	if svc.state != StateUnknown {
		return AlreadyStarted
	}
	if svc.Log == nil || svc.Bus == nil {
		return NotConfigured
	}
	err := svc.Bus.Dial()
	if err != nil {
		return err
	}
	ch := svc.Bus.GrabName(true)
	log := svc.Log
	go func() {
		for err := range ch {
			if !svc.IsRunning() {
				break
			}
			if err != nil {
				log.Fatalf("name channel for %s got: %v",
					busAddr.Name, err)
			}
		}
	}()
	svc.Bus.WatchMethod(dispatchMap, svc)
	svc.state = StateRunning
	return nil
}

// Stop() closes the bus and sets the state to StateFinished
func (svc *DBusService) Stop() {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	if svc.Bus != nil {
		svc.Bus.Close()
	}
	svc.state = StateFinished
}

