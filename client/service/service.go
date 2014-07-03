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
	"net/url"
	"os"
	"strings"

	"launchpad.net/ubuntu-push/bus"
	http13 "launchpad.net/ubuntu-push/http13client"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/nih"
)

// PushServiceSetup encapsulates the params for setting up a PushService.
type PushServiceSetup struct {
	RegURL     *url.URL
	DeviceId   string
	AuthGetter func(string) string
}

// PushService is the dbus api
type PushService struct {
	DBusService
	regURL     *url.URL
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
func NewPushService(bus bus.Endpoint, setup *PushServiceSetup, log logger.Logger) *PushService {
	var svc = &PushService{}
	svc.Log = log
	svc.Bus = bus
	svc.regURL = setup.RegURL
	svc.deviceId = setup.DeviceId
	svc.authGetter = setup.AuthGetter
	return svc
}

// getAuthorization() returns the URL and the authorization header for
// POSTing to the registration HTTP endpoint for op
func (svc *PushService) getAuthorization(op string) (string, string) {
	if svc.authGetter == nil || svc.regURL == nil {
		return "", ""
	}
	purl, err := svc.regURL.Parse(op)
	if err != nil {
		panic("op to getAuthorization was invalid")
	}
	url := purl.String()
	return url, svc.authGetter(url)
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

func (svc *PushService) manageReg(op, appId string) (*registrationReply, error) {
	req_body, err := json.Marshal(registrationRequest{svc.deviceId, appId})
	if err != nil {
		return nil, fmt.Errorf("unable to marshal register request body: %v", err)
	}

	url, auth := svc.getAuthorization(op)
	if auth == "" {
		return nil, BadAuth
	}

	req, err := http13.NewRequest("POST", url, bytes.NewReader(req_body))
	if err != nil {
		panic(fmt.Errorf("unable to build register request: %v", err))
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

	return &reply, nil
}

func (svc *PushService) register(path string, args, _ []interface{}) ([]interface{}, error) {
	if len(args) != 0 {
		return nil, BadArgCount
	}
	raw_appname := path[strings.LastIndex(path, "/")+1:]
	appname := string(nih.Unquote([]byte(raw_appname)))

	rv := os.Getenv("PUSH_REG_" + raw_appname)
	if rv != "" {
		return []interface{}{rv}, nil
	}

	reply, err := svc.manageReg("/register", appname)
	if err != nil {
		return nil, err
	}

	if !reply.Ok || reply.Token == "" {
		svc.Log.Errorf("Unexpected response: %#v", reply)
		return nil, BadToken
	}

	return []interface{}{reply.Token}, nil
}

func (svc *PushService) Unregister(appId string) error {
	_, err := svc.manageReg("/unregister", appId)
	return err
}
