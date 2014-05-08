package service

import (
	"errors"
	"sync"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/logger"
)

type Service struct {
	lock      sync.RWMutex
	isStarted bool
	Log       logger.Logger
	Bus       bus.Endpoint
}

var (
	NotConfigured  = errors.New("not configured")
	AlreadyStarted = errors.New("already started")
	BusAddress     = bus.Address{
		Interface: "com.ubuntu.PushNotifications",
		Path:      "com/ubuntu/PushNotifications",
		Name:      "com.ubuntu.PushNotifications",
	}
)

func (svc *Service) IsStarted() bool {
	svc.lock.RLock()
	defer svc.lock.RUnlock()
	return svc.isStarted
}

func (svc *Service) Start() error {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	if svc.isStarted {
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
	svc.isStarted = true
	return nil
}
