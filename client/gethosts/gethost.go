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

// Package gethosts implements finding hosts to connect to for delivery of notifications.
package gethosts

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"launchpad.net/ubuntu-push/external/murmur3"
	http13 "launchpad.net/ubuntu-push/http13client"
)

// GetHost implements finding hosts to connect to consulting a remote endpoint providing a hash of the device identifier.
type GetHost struct {
	hash        string
	endpointUrl string
	cli         *http13.Client
}

// New makes a GetHost.
func New(deviceId, endpointUrl string, timeout time.Duration) *GetHost {
	hash := murmur3.Sum64([]byte(deviceId))
	return &GetHost{
		hash:        fmt.Sprintf("%x", hash),
		endpointUrl: endpointUrl,
		cli: &http13.Client{
			Transport: &http13.Transport{TLSHandshakeTimeout: timeout},
			Timeout:   timeout,
		},
	}
}

type Host struct {
	Domain string
	Hosts  []string
}

var (
	ErrRequest   = errors.New("request was not accepted")
	ErrInternal  = errors.New("remote had an internal error")
	ErrTemporary = errors.New("remote had a temporary error")
)

// Get gets a list of hosts consulting the endpoint.
func (gh *GetHost) Get() (*Host, error) {
	resp, err := gh.cli.Get(gh.endpointUrl + "?h=" + gh.hash)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		switch {
		case resp.StatusCode == http.StatusInternalServerError:
			return nil, ErrInternal
		case resp.StatusCode > http.StatusInternalServerError:
			return nil, ErrTemporary
		default:
			return nil, ErrRequest
		}
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var parsed Host
	err = json.Unmarshal(body, &parsed)
	if err != nil {
		return nil, ErrTemporary
	}
	if len(parsed.Hosts) == 0 {
		return nil, ErrTemporary
	}
	return &parsed, nil
}
