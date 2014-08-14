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

// Package kit contains reusable building blocks for acceptance.
package kit

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

// APIClient helps making api requests.
type APIClient struct {
	ServerAPIURL string
	// hook to adjust requests
	MassageRequest func(req *http.Request, message interface{}) *http.Request
	// other state
	httpClient *http.Client
}

// SetupClient sets up the http client to make requests.
func (api *APIClient) SetupClient() {
	api.httpClient = &http.Client{}
}

// Post a API request.
func (api *APIClient) PostRequest(path string, message interface{}) (string, error) {
	packedMessage, err := json.Marshal(message)
	if err != nil {
		panic(err)
	}
	reader := bytes.NewReader(packedMessage)

	url := api.ServerAPIURL + path
	request, _ := http.NewRequest("POST", url, reader)
	request.ContentLength = int64(reader.Len())
	request.Header.Set("Content-Type", "application/json")

	if api.MassageRequest != nil {
		request = api.MassageRequest(request, message)
	}

	resp, err := api.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return string(body), err
}
