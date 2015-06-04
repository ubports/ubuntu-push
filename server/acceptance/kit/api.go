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
	_ "crypto/sha512" // support sha384/512 certs
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

// APIClient helps making api requests.
type APIClient struct {
	ServerAPIURL string
	// hook to adjust requests
	MassageRequest func(req *http.Request, message interface{}) *http.Request
	// other state
	httpClient *http.Client
}

type APIError struct {
	Msg  string
	Body []byte
}

func (e *APIError) Error() string {
	return e.Msg
}

// SetupClient sets up the http client to make requests.
func (api *APIClient) SetupClient(tlsConfig *tls.Config, disableKeepAlives bool, maxIdleConnsPerHost int) {
	api.httpClient = &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsConfig,
			DisableKeepAlives:   disableKeepAlives,
			MaxIdleConnsPerHost: maxIdleConnsPerHost},
	}
}

var ErrNOk = errors.New("not ok")

func readBody(respBody io.ReadCloser) (map[string]interface{}, error) {
	defer respBody.Close()
	body, err := ioutil.ReadAll(respBody)
	if err != nil {
		return nil, err
	}
	var res map[string]interface{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, &APIError{err.Error(), body}
	}
	return res, nil
}

// Post a API request.
func (api *APIClient) PostRequest(path string, message interface{}) (map[string]interface{}, error) {
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
		return nil, err
	}
	res, err := readBody(resp.Body)
	if err != nil {
		return nil, err
	}

	if ok, _ := res["ok"].(bool); !ok {
		return res, ErrNOk
	}
	return res, nil
}

// Get resource from API endpoint.
func (api *APIClient) GetRequest(path string, params map[string]string) (map[string]interface{}, error) {
	apiURL := api.ServerAPIURL + path
	if len(params) != 0 {
		vals := url.Values{}
		for k, v := range params {
			vals.Set(k, v)
		}
		apiURL += "?" + vals.Encode()
	}
	request, _ := http.NewRequest("GET", apiURL, nil)

	resp, err := api.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	res, err := readBody(resp.Body)
	if err != nil {
		return nil, err
	}
	return res, nil
}
