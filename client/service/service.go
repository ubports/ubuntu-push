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

package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"launchpad.net/ubuntu-push/bus"
	http13 "launchpad.net/ubuntu-push/http13client"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/nih"
)

// PushService is the dbus api
type PushService struct {
	DBusService
	regURL     string
	deviceId   string
	authGetter func(string) string
	httpCli    http13.Client
}

var (
	PushServiceBusAddress = bus.Address{
		Interface: "com.ubuntu.PushNotifications",
		Path:      "/com/ubuntu/PushNotifications",
		Name:      "com.ubuntu.PushNotifications",
	}
)

// NewPushService() builds a new service and returns it.
func NewPushService(bus bus.Endpoint, log logger.Logger) *PushService {
	var svc = &PushService{}
	svc.Log = log
	svc.Bus = bus
	return svc
}

// SetRegistrationURL() sets the registration url for the service
func (svc *PushService) SetRegistrationURL(url string) {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	svc.regURL = url
}

// SetAuthGetter() sets the authorization getter for the service
func (svc *PushService) SetAuthGetter(authGetter func(string) string) {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	svc.authGetter = authGetter
}

// getRegistrationAuthorization() returns the authorization header for
// POSTing to the registration HTTP endpoint
//
// (this is for calling with the lock held)
func (svc *PushService) getRegistrationAuthorization() string {
	if svc.authGetter != nil && svc.regURL != "" {
		return svc.authGetter(svc.regURL)
	} else {
		return ""
	}
}

// GetRegistrationAuthorization() returns the authorization header for
// POSTing to the registration HTTP endpoint
func (svc *PushService) GetRegistrationAuthorization() string {
	svc.lock.RLock()
	defer svc.lock.RUnlock()
	return svc.getRegistrationAuthorization()
}

// SetDeviceId() sets the device id
func (svc *PushService) SetDeviceId(deviceId string) {
	svc.lock.Lock()
	defer svc.lock.Unlock()
	svc.deviceId = deviceId
}

// GetDeviceId() returns the device id
func (svc *PushService) GetDeviceId() string {
	svc.lock.RLock()
	defer svc.lock.RUnlock()
	return svc.deviceId
}

func (svc *PushService) Start() error {
	return svc.DBusService.Start(bus.DispatchMap{
		"Register": svc.register,
	}, PushServiceBusAddress)
}

var (
	BadServer  = errors.New("Bad server")
	BadRequest = errors.New("Bad request")
	BadToken   = errors.New("Bad token")
	BadAuth    = errors.New("Bad auth")
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

func (svc *PushService) register(path string, args, _ []interface{}) ([]interface{}, error) {
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
	req, err := http13.NewRequest("POST", svc.regURL, bytes.NewReader(req_body))
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
	if resp.StatusCode != http.StatusOK {
		svc.Log.Errorf("register endpoint replied %d", resp.StatusCode)
		switch {
		case resp.StatusCode >= http.StatusInternalServerError:
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
