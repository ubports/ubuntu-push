package service

import (
	"errors"
	"os"
	"sync"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/logger"
)

type Service struct {
	lock  sync.RWMutex
	state ServiceState
	Log   logger.Logger
	Bus   bus.Endpoint
}

type ServiceState uint8

const (
	StateUnknown ServiceState = iota
	StateRunning
	StateFinished
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

func (svc *Service) IsRunning() bool {
	svc.lock.RLock()
	defer svc.lock.RUnlock()
	return svc.state == StateRunning
}

func (svc *Service) Start() error {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	if svc.state != StateUnknown {
		return AlreadyStarted
	}
	if svc.Log == nil {
		return NotConfigured
	}
	if svc.Bus == nil {
		svc.Bus = bus.SessionBus.Endpoint(BusAddress, svc.Log)
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
				log.Fatalf("name channel for %s got: %v", BusAddress.Name, err)
			}
		}
	}()
	svc.Bus.WatchMethod(bus.DispatchMap{
		"Register": Register,
	})
	svc.state = StateRunning
	return nil
}

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

func Register(args []interface{}, _ []interface{}) ([]interface{}, error) {
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
