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
	mbox  map[string][]string
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

func NewService(bus bus.Endpoint, log logger.Logger) *Service {
	return &Service{Log: log, Bus: bus}
}

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
				log.Fatalf("name channel for %s got: %v", BusAddress.Name, err)
			}
		}
	}()
	svc.Bus.WatchMethod(bus.DispatchMap{
		"Register":      Register,
		"Notifications": Notifications,
		"Inject":        Inject,
	}, svc)
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

func Notifications(args []interface{}, extra []interface{}) ([]interface{}, error) {
	if len(args) != 1 {
		return nil, BadArgCount
	}
	appname, ok := args[0].(string)
	if !ok {
		return nil, BadArgType
	}

	svc := extra[0].(*Service)

	svc.lock.Lock()
	defer svc.lock.Unlock()

	if svc.mbox == nil {
		return []interface{}{[]string(nil)}, nil
	}
	msgs := svc.mbox[appname]
	delete(svc.mbox, appname)

	return []interface{}{msgs}, nil
}

func Inject(args []interface{}, extra []interface{}) ([]interface{}, error) {
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

	svc := extra[0].(*Service)

	svc.lock.Lock()
	defer svc.lock.Unlock()
	if svc.mbox == nil {
		svc.mbox = make(map[string][]string)
	}
	svc.mbox[appname] = append(svc.mbox[appname], notif)

	svc.Bus.Signal("Notification", []interface{}{appname})

	return nil, nil
}
