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

// Package api has code that offers a REST API for the applications that
// want to push messages.
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/server/broker"
	"launchpad.net/ubuntu-push/server/store"
)

const MaxRequestBodyBytes = 4 * 1024
const JSONMediaType = "application/json"

// APIError represents a API error (both internally and as JSON in a response).
type APIError struct {
	// http status code
	StatusCode int `json:"-"`
	// machine readable label
	ErrorLabel string `json:"error"`
	// human message
	Message string `json:"message"`
}

// machine readable error labels
const (
	ioError        = "io-error"
	invalidRequest = "invalid-request"
	unknownChannel = "unknown channel"
	unavailable    = "unavailable"
	internalError  = "internal"
)

func (apiErr *APIError) Error() string {
	return fmt.Sprintf("api %s: %s", apiErr.ErrorLabel, apiErr.Message)
}

// Well-known prebuilt API errors
var (
	ErrNoContentLengthProvided = &APIError{
		http.StatusLengthRequired,
		invalidRequest,
		"A Content-Length must be provided",
	}
	ErrRequestBodyEmpty = &APIError{
		http.StatusBadRequest,
		invalidRequest,
		"Request body empty",
	}
	ErrRequestBodyTooLarge = &APIError{
		http.StatusRequestEntityTooLarge,
		invalidRequest,
		"Request body too large",
	}
	ErrWrongContentType = &APIError{
		http.StatusUnsupportedMediaType,
		invalidRequest,
		"Wrong content type, should be application/json",
	}
	ErrWrongRequestMethod = &APIError{
		http.StatusMethodNotAllowed,
		invalidRequest,
		"Wrong request method, should be POST",
	}
	ErrMalformedJSONObject = &APIError{
		http.StatusBadRequest,
		invalidRequest,
		"Malformed JSON Object",
	}
	ErrCouldNotReadBody = &APIError{
		http.StatusBadRequest,
		ioError,
		"Could not read request body",
	}
	ErrMissingData = &APIError{
		http.StatusBadRequest,
		invalidRequest,
		"Missing data field",
	}
	ErrUnknownChannel = &APIError{
		http.StatusBadRequest,
		unknownChannel,
		"Unknown channel",
	}
	ErrUnknown = &APIError{
		http.StatusInternalServerError,
		internalError,
		"Unknown error",
	}
	ErrCouldNotStoreNotification = &APIError{
		http.StatusServiceUnavailable,
		unavailable,
		"Could not store notification",
	}
)

type Message struct {
	Registration string          `json:"registration"`
	CoalesceTag  string          `json:"coalesce_tag"`
	Data         json.RawMessage `json:"data"`
}

// Broadcast request JSON object.
type Broadcast struct {
	Channel     string          `json:"channel"`
	ExpireAfter uint8           `json:"expire_after"`
	Data        json.RawMessage `json:"data"`
}

func respondError(writer http.ResponseWriter, apiErr *APIError) {
	wireError, err := json.Marshal(apiErr)
	if err != nil {
		panic(fmt.Errorf("couldn't marshal our own errors: %v", err))
	}
	writer.Header().Set("Content-type", JSONMediaType)
	writer.WriteHeader(apiErr.StatusCode)
	writer.Write(wireError)
}

func checkContentLength(request *http.Request) *APIError {
	if request.ContentLength == -1 {
		return ErrNoContentLengthProvided
	}
	if request.ContentLength == 0 {
		return ErrRequestBodyEmpty
	}
	if request.ContentLength > MaxRequestBodyBytes {
		return ErrRequestBodyTooLarge
	}
	return nil
}

func checkRequestAsPost(request *http.Request) *APIError {
	if err := checkContentLength(request); err != nil {
		return err
	}
	if request.Header.Get("Content-Type") != JSONMediaType {
		return ErrWrongContentType
	}
	if request.Method != "POST" {
		return ErrWrongRequestMethod
	}
	return nil
}

func readBody(request *http.Request) ([]byte, *APIError) {
	if err := checkRequestAsPost(request); err != nil {
		return nil, err
	}

	body := make([]byte, request.ContentLength)
	_, err := io.ReadFull(request.Body, body)

	if err != nil {
		return nil, ErrCouldNotReadBody
	}

	return body, nil
}

func checkBroadcast(bcast *Broadcast) *APIError {
	if len(bcast.Data) == 0 {
		return ErrMissingData
	}
	return nil
}

// state holds the interfaces to delegate to serving requests
type state struct {
	store  store.PendingStore
	broker broker.BrokerSending
	logger logger.Logger
}

type BroadcastHandler state

func (h *BroadcastHandler) doBroadcast(bcast *Broadcast) *APIError {
	apiErr := checkBroadcast(bcast)
	if apiErr != nil {
		return apiErr
	}
	chanId, err := h.store.GetInternalChannelId(bcast.Channel)
	if err != nil {
		switch err {
		case store.ErrUnknownChannel:
			return ErrUnknownChannel
		default:
			return ErrUnknown
		}
	}
	// xxx ignoring expiration for now
	err = h.store.AppendToChannel(chanId, bcast.Data)
	if err != nil {
		// assume this for now
		return ErrCouldNotStoreNotification
	}

	h.broker.Broadcast(chanId)
	return nil
}

func (h *BroadcastHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	body, apiErr := readBody(request)

	if apiErr != nil {
		respondError(writer, apiErr)
		return
	}

	broadcast := &Broadcast{}
	err := json.Unmarshal(body, broadcast)

	if err != nil {
		respondError(writer, ErrMalformedJSONObject)
		return
	}

	apiErr = h.doBroadcast(broadcast)
	if apiErr != nil {
		respondError(writer, apiErr)
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(writer, `{"ok":true}`)
}

// MakeHandlersMux makes a handler that dispatches for the various API endpoints.
func MakeHandlersMux(store store.PendingStore, broker broker.BrokerSending, logger logger.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/broadcast", &BroadcastHandler{
		store:  store,
		broker: broker,
		logger: logger,
	})
	return mux
}
