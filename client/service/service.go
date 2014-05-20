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
	"errors"
	"os"
	"sync"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/logger"
)

// Service is the dbus api
type Service struct {
	lock       sync.RWMutex
	state      ServiceState
	mbox       map[string][][]byte
	msgHandler func([]byte) error
	Log        logger.Logger
	Bus        bus.Endpoint
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
	BusAddress     = bus.Address{
		Interface: "com.ubuntu.PushNotifications",
		Path:      "/com/ubuntu/PushNotifications",
		Name:      "com.ubuntu.PushNotifications",
	}
)

// NewService() builds a new service and returns it.
func NewService(bus bus.Endpoint, log logger.Logger) *Service {
	return &Service{Log: log, Bus: bus}
}

// SetMessageHandler() sets the message-handling callback
func (svc *Service) SetMessageHandler(callback func([]byte) error) {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	svc.msgHandler = callback
}

// GetMessageHandler() returns the (possibly nil) messaging handler callback
func (svc *Service) GetMessageHandler() func([]byte) error {
	svc.lock.RLock()
	defer svc.lock.RUnlock()
	return svc.msgHandler
}

// IsRunning() returns whether the service's state is StateRunning
func (svc *Service) IsRunning() bool {
	svc.lock.RLock()
	defer svc.lock.RUnlock()
	return svc.state == StateRunning
}

// Start() dials the bus, grab the name, and listens for method calls.
func (svc *Service) Start() error {
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
					BusAddress.Name, err)
			}
		}
	}()
	svc.Bus.WatchMethod(bus.DispatchMap{
		"Register":      svc.register,
		"Notifications": svc.notifications,
		"Inject":        svc.inject,
	}, svc)
	svc.state = StateRunning
	return nil
}

// Stop() closes the bus and sets the state to StateFinished
func (svc *Service) Stop() {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	if svc.Bus != nil {
		svc.Bus.Close()
	}
	svc.state = StateFinished
}

var (
	BadArgCount = errors.New("Wrong number of arguments")
	BadArgType  = errors.New("Bad argument type")
)

func (svc *Service) register(args []interface{}, _ []interface{}) ([]interface{}, error) {
	if len(args) != 1 {
		return nil, BadArgCount
	}
	appname, ok := args[0].(string)
	if !ok {
		return nil, BadArgType
	}

	rv := os.Getenv("PUSH_REG_" + appname)
	if rv == "" {
		rv = "this-is-an-opaque-block-of-random-bits-i-promise"
	}

	return []interface{}{rv}, nil
}

func (svc *Service) notifications(args []interface{}, _ []interface{}) ([]interface{}, error) {
	if len(args) != 1 {
		return nil, BadArgCount
	}
	appname, ok := args[0].(string)
	if !ok {
		return nil, BadArgType
	}

	svc.lock.Lock()
	defer svc.lock.Unlock()

	if svc.mbox == nil {
		return []interface{}{[]string(nil)}, nil
	}
	msgs := svc.mbox[appname]
	delete(svc.mbox, appname)

	return []interface{}{msgs}, nil
}

func (svc *Service) inject(args []interface{}, _ []interface{}) ([]interface{}, error) {
	if len(args) != 2 {
		return nil, BadArgCount
	}
	appname, ok := args[0].(string)
	if !ok {
		return nil, BadArgType
	}
	notif, ok := args[1].(string)
	if !ok {
		return nil, BadArgType
	}

	svc.Inject(appname, []byte(notif))

	return nil, nil
}

// Inject() signals to an application over dbus that a notification
// has arrived.
func (svc *Service) Inject(appname string, notif []byte) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	if svc.mbox == nil {
		svc.mbox = make(map[string][][]byte)
	}
	svc.mbox[appname] = append(svc.mbox[appname], notif)
	if svc.msgHandler != nil {
		err := svc.msgHandler(notif)
		if err != nil {
			svc.Log.Errorf("msgHandler returned %v", err)
			return err
		}
		svc.Log.Debugf("call to msgHandler successful")
	}

	return svc.Bus.Signal("Notification", []interface{}{appname})
}
