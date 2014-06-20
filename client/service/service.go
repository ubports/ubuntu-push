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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	http_old "net/http"
	"os"
	"strings"
	"sync"

	"launchpad.net/ubuntu-push/bus"
	http "launchpad.net/ubuntu-push/http13client"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/nih"
)

// Service is the dbus api
type Service struct {
	lock       sync.RWMutex
	state      ServiceState
	mbox       map[string][]string
	msgHandler func([]byte) error
	regURL     string
	deviceId   string
	authGetter func(string) string
	httpCli    http.Client
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
		Path:      "/com/ubuntu/PushNotifications/*",
		Name:      "com.ubuntu.PushNotifications",
	}
)

// NewService() builds a new service and returns it.
func NewService(bus bus.Endpoint, log logger.Logger) *Service {
	return &Service{Log: log, Bus: bus}
}

// SetRegistrationURL() sets the registration url for the service
func (svc *Service) SetRegistrationURL(url string) {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	svc.regURL = url
}

// SetAuthGetter() sets the authorization getter for the service
func (svc *Service) SetAuthGetter(authGetter func(string) string) {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	svc.authGetter = authGetter
}

// getRegistrationAuthorization() returns the authorization header for
// POSTing to the registration HTTP endpoint
//
// (this is for calling with the lock held)
func (svc *Service) getRegistrationAuthorization() string {
	if svc.authGetter != nil && svc.regURL != "" {
		return svc.authGetter(svc.regURL)
	} else {
		return ""
	}
}

// GetRegistrationAuthorization() returns the authorization header for
// POSTing to the registration HTTP endpoint
func (svc *Service) GetRegistrationAuthorization() string {
	svc.lock.RLock()
	defer svc.lock.RUnlock()
	return svc.getRegistrationAuthorization()
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

// SetDeviceId() sets the device id
func (svc *Service) SetDeviceId(deviceId string) {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	svc.deviceId = deviceId
}

// GetDeviceId() returns the device id
func (svc *Service) GetDeviceId() string {
	svc.lock.RLock()
	defer svc.lock.RUnlock()
	return svc.deviceId
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
	BadServer   = errors.New("Bad server")
	BadRequest  = errors.New("Bad request")
	BadToken    = errors.New("Bad token")
	BadAuth     = errors.New("Bad auth")
)

type registrationRequest struct {
	DeviceId string `json:"deviceid"`
	AppId    string `json:"appid"`
}

type registrationReply struct {
	Token   string `json:"token"`   // the bit we're after
	Ok      bool   `json:"ok"`      // only ever true or absent
	Error   string `json:"error"`   // these two only used for debugging
	Message string `json:"message"` //
}

func (svc *Service) register(path string, args, _ []interface{}) ([]interface{}, error) {
	svc.lock.RLock()
	defer svc.lock.RUnlock()

	if len(args) != 0 {
		return nil, BadArgCount
	}
	raw_appname := path[strings.LastIndex(path, "/")+1:]
	appname := string(nih.Unquote([]byte(raw_appname)))

	rv := os.Getenv("PUSH_REG_" + raw_appname)
	if rv != "" {
		return []interface{}{rv}, nil
	}

	req_body, err := json.Marshal(registrationRequest{svc.deviceId, appname})
	if err != nil {
		return nil, fmt.Errorf("unable to marshal register request body: %v", err)
	}
	req, err := http.NewRequest("POST", svc.regURL, bytes.NewReader(req_body))
	if err != nil {
		return nil, fmt.Errorf("unable to build register request: %v", err)
	}
	auth := svc.getRegistrationAuthorization()
	if auth == "" {
		return nil, BadAuth
	}
	req.Header.Add("Authorization", auth)
	req.Header.Add("Content-Type", "application/json")

	resp, err := svc.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to request registration: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http_old.StatusOK {
		svc.Log.Errorf("register endpoint replied %d", resp.StatusCode)
		switch {
		case resp.StatusCode >= http_old.StatusInternalServerError:
			// XXX retry on 503
			return nil, BadServer
		default:
			return nil, BadRequest
		}
	}
	// errors below here Can't Happen (tm).
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		svc.Log.Errorf("Reading response body: %v", err)
		return nil, err
	}

	var reply registrationReply
	err = json.Unmarshal(body, &reply)
	if err != nil {
		svc.Log.Errorf("Unmarshalling response body: %v", err)
		return nil, fmt.Errorf("unable to unmarshal register response: %v", err)
	}

	if !reply.Ok || reply.Token == "" {
		svc.Log.Errorf("Unexpected response: %#v", reply)
		return nil, BadToken
	}

	return []interface{}{reply.Token}, nil
}

func (svc *Service) notifications(path string, args, _ []interface{}) ([]interface{}, error) {
	if len(args) != 0 {
		return nil, BadArgCount
	}
	appname := string(nih.Unquote([]byte(path[strings.LastIndex(path, "/")+1:])))

	svc.lock.Lock()
	defer svc.lock.Unlock()

	if svc.mbox == nil {
		return []interface{}{[]string(nil)}, nil
	}
	msgs := svc.mbox[appname]
	delete(svc.mbox, appname)

	return []interface{}{msgs}, nil
}

func (svc *Service) inject(path string, args, _ []interface{}) ([]interface{}, error) {
	if len(args) != 1 {
		return nil, BadArgCount
	}
	notif, ok := args[0].(string)
	if !ok {
		return nil, BadArgType
	}
	appname := string(nih.Unquote([]byte(path[strings.LastIndex(path, "/")+1:])))

	return nil, svc.Inject(appname, notif)
}

// Inject() signals to an application over dbus that a notification
// has arrived.
func (svc *Service) Inject(appname string, notif string) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	if svc.mbox == nil {
		svc.mbox = make(map[string][]string)
	}
	svc.mbox[appname] = append(svc.mbox[appname], notif)
	if svc.msgHandler != nil {
		err := svc.msgHandler([]byte(notif))
		if err != nil {
			svc.Log.Errorf("msgHandler returned %v", err)
			return err
		}
		svc.Log.Debugf("call to msgHandler successful")
	}

	return svc.Bus.Signal("Notification", []interface{}{appname})
}
