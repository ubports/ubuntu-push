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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"code.google.com/p/go-uuid/uuid"

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
	unknownChannel = "unknown-channel"
	unknownToken   = "unknown-token"
	unauthorized   = "unauthorized"
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
	ErrMissingIdField = &APIError{
		http.StatusBadRequest,
		invalidRequest,
		"Missing id field",
	}
	ErrMissingData = &APIError{
		http.StatusBadRequest,
		invalidRequest,
		"Missing data field",
	}
	ErrInvalidExpiration = &APIError{
		http.StatusBadRequest,
		invalidRequest,
		"Invalid expiration date",
	}
	ErrPastExpiration = &APIError{
		http.StatusBadRequest,
		invalidRequest,
		"Past expiration date",
	}
	ErrUnknownChannel = &APIError{
		http.StatusBadRequest,
		unknownChannel,
		"Unknown channel",
	}
	ErrUnknownToken = &APIError{
		http.StatusBadRequest,
		unknownToken,
		"Unknown token",
	}
	ErrUnknown = &APIError{
		http.StatusInternalServerError,
		internalError,
		"Unknown error",
	}
	ErrStoreUnavailable = &APIError{
		http.StatusServiceUnavailable,
		unavailable,
		"Message store unavailable",
	}
	ErrCouldNotStoreNotification = &APIError{
		http.StatusServiceUnavailable,
		unavailable,
		"Could not store notification",
	}
	ErrCouldNotMakeToken = &APIError{
		http.StatusServiceUnavailable,
		unavailable,
		"Could not make token",
	}
	ErrCouldNotResolveToken = &APIError{
		http.StatusServiceUnavailable,
		unavailable,
		"Could not resolve token",
	}
	ErrUnauthorized = &APIError{
		http.StatusUnauthorized,
		unauthorized,
		"Unauthorized",
	}
)

type Registration struct {
	DeviceId string `json:"deviceid"`
	AppId    string `json:"appid"`
}

type Unicast struct {
	Token    string `json:"token"`
	UserId   string `json:"userid"`
	DeviceId string `json:"deviceid"`
	AppId    string `json:"appid"`
	//CoalesceTag  string          `json:"coalesce_tag"`
	ExpireOn string          `json:"expire_on"`
	Data     json.RawMessage `json:"data"`
}

// Broadcast request JSON object.
type Broadcast struct {
	Channel  string          `json:"channel"`
	ExpireOn string          `json:"expire_on"`
	Data     json.RawMessage `json:"data"`
}

// RespondError writes back a JSON error response for a APIError.
func RespondError(writer http.ResponseWriter, apiErr *APIError) {
	wireError, err := json.Marshal(apiErr)
	if err != nil {
		panic(fmt.Errorf("couldn't marshal our own errors: %v", err))
	}
	writer.Header().Set("Content-type", JSONMediaType)
	writer.WriteHeader(apiErr.StatusCode)
	writer.Write(wireError)
}

func checkContentLength(request *http.Request, maxBodySize int64) *APIError {
	if request.ContentLength == -1 {
		return ErrNoContentLengthProvided
	}
	if request.ContentLength == 0 {
		return ErrRequestBodyEmpty
	}
	if request.ContentLength > maxBodySize {
		return ErrRequestBodyTooLarge
	}
	return nil
}

func checkRequestAsPost(request *http.Request, maxBodySize int64) *APIError {
	if request.Method != "POST" {
		return ErrWrongRequestMethod
	}
	if err := checkContentLength(request, maxBodySize); err != nil {
		return err
	}
	if request.Header.Get("Content-Type") != JSONMediaType {
		return ErrWrongContentType
	}
	return nil
}

// ReadBody checks that a POST request is well-formed and reads its body.
func ReadBody(request *http.Request, maxBodySize int64) ([]byte, *APIError) {
	if err := checkRequestAsPost(request, maxBodySize); err != nil {
		return nil, err
	}

	body := make([]byte, request.ContentLength)
	_, err := io.ReadFull(request.Body, body)

	if err != nil {
		return nil, ErrCouldNotReadBody
	}

	return body, nil
}

var zeroTime = time.Time{}

func checkCastCommon(data json.RawMessage, expireOn string) (time.Time, *APIError) {
	if len(data) == 0 {
		return zeroTime, ErrMissingData
	}
	expire, err := time.Parse(time.RFC3339, expireOn)
	if err != nil {
		return zeroTime, ErrInvalidExpiration
	}
	if expire.Before(time.Now()) {
		return zeroTime, ErrPastExpiration
	}
	return expire, nil
}

func checkBroadcast(bcast *Broadcast) (time.Time, *APIError) {
	return checkCastCommon(bcast.Data, bcast.ExpireOn)
}

type StoreForRequest func(w http.ResponseWriter, request *http.Request) (store.PendingStore, error)

// context holds the interfaces to delegate to serving requests
type context struct {
	storeForRequest StoreForRequest
	broker          broker.BrokerSending
	logger          logger.Logger
}

func (ctx *context) getStore(w http.ResponseWriter, request *http.Request) (store.PendingStore, *APIError) {
	sto, err := ctx.storeForRequest(w, request)
	if err != nil {
		apiErr, ok := err.(*APIError)
		if ok {
			return nil, apiErr
		}
		ctx.logger.Errorf("failed to get store: %v", err)
		return nil, ErrUnknown
	}
	return sto, nil
}

func (ctx *context) prepare(w http.ResponseWriter, request *http.Request, reqObj interface{}) (store.PendingStore, *APIError) {
	body, apiErr := ReadBody(request, MaxRequestBodyBytes)
	if apiErr != nil {
		return nil, apiErr
	}

	err := json.Unmarshal(body, reqObj)
	if err != nil {
		return nil, ErrMalformedJSONObject
	}

	return ctx.getStore(w, request)
}

type BroadcastHandler struct {
	*context
}

func (h *BroadcastHandler) doBroadcast(sto store.PendingStore, bcast *Broadcast) *APIError {
	expire, apiErr := checkBroadcast(bcast)
	if apiErr != nil {
		return apiErr
	}
	chanId, err := sto.GetInternalChannelId(bcast.Channel)
	if err != nil {
		switch err {
		case store.ErrUnknownChannel:
			return ErrUnknownChannel
		default:
			return ErrUnknown
		}
	}
	err = sto.AppendToChannel(chanId, bcast.Data, expire)
	if err != nil {
		h.logger.Errorf("could not store notification: %v", err)
		return ErrCouldNotStoreNotification
	}

	h.broker.Broadcast(chanId)
	return nil
}

func (h *BroadcastHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var apiErr *APIError
	defer func() {
		if apiErr != nil {
			RespondError(writer, apiErr)
		}
	}()

	broadcast := &Broadcast{}

	sto, apiErr := h.prepare(writer, request, broadcast)
	if apiErr != nil {
		return
	}
	defer sto.Close()

	apiErr = h.doBroadcast(sto, broadcast)
	if apiErr != nil {
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(writer, `{"ok":true}`)
}

type UnicastHandler struct {
	*context
}

func checkUnicast(ucast *Unicast) (time.Time, *APIError) {
	if ucast.AppId == "" {
		return zeroTime, ErrMissingIdField
	}
	if ucast.Token == "" && (ucast.UserId == "" || ucast.DeviceId == "") {
		return zeroTime, ErrMissingIdField
	}
	return checkCastCommon(ucast.Data, ucast.ExpireOn)
}

// use a base64 encoded TimeUUID
var generateMsgId = func() string {
	return base64.StdEncoding.EncodeToString(uuid.NewUUID())
}

func (h *UnicastHandler) doUnicast(sto store.PendingStore, ucast *Unicast) *APIError {
	expire, apiErr := checkUnicast(ucast)
	if apiErr != nil {
		return apiErr
	}
	chanId, err := sto.GetInternalChannelIdFromToken(ucast.Token, ucast.AppId, ucast.UserId, ucast.DeviceId)
	if err != nil {
		switch err {
		case store.ErrUnknownToken:
			return ErrUnknownToken
		case store.ErrUnauthorized:
			return ErrUnauthorized
		default:
			h.logger.Errorf("could not resolve token: %v", err)
			return ErrCouldNotResolveToken
		}
	}

	msgId := generateMsgId()
	err = sto.AppendToUnicastChannel(chanId, ucast.AppId, ucast.Data, msgId, expire)
	if err != nil {
		h.logger.Errorf("could not store notification: %v", err)
		return ErrCouldNotStoreNotification
	}

	h.broker.Unicast(chanId)
	return nil
}

func (h *UnicastHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var apiErr *APIError
	defer func() {
		if apiErr != nil {
			RespondError(writer, apiErr)
		}
	}()

	unicast := &Unicast{}

	sto, apiErr := h.prepare(writer, request, unicast)
	if apiErr != nil {
		return
	}
	defer sto.Close()

	apiErr = h.doUnicast(sto, unicast)
	if apiErr != nil {
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(writer, `{"ok":true}`)
}

type RegisterHandler struct {
	*context
}

func checkRegister(reg *Registration) *APIError {
	if reg.DeviceId == "" || reg.AppId == "" {
		return ErrMissingIdField
	}
	return nil
}

func (h *RegisterHandler) doRegister(sto store.PendingStore, reg *Registration) (string, *APIError) {
	apiErr := checkRegister(reg)
	if apiErr != nil {
		return "", apiErr
	}
	token, err := sto.Register(reg.DeviceId, reg.AppId)
	if err != nil {
		h.logger.Errorf("could not make a token: %v", err)
		return "", ErrCouldNotMakeToken
	}
	return token, nil
}

func (h *RegisterHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var apiErr *APIError
	defer func() {
		if apiErr != nil {
			RespondError(writer, apiErr)
		}
	}()

	reg := &Registration{}

	sto, apiErr := h.prepare(writer, request, reg)
	if apiErr != nil {
		return
	}
	defer sto.Close()

	token, apiErr := h.doRegister(sto, reg)
	if apiErr != nil {
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	res, err := json.Marshal(map[string]interface{}{
		"ok":    true,
		"token": token,
	})
	if err != nil {
		panic(fmt.Errorf("couldn't marshal our own response: %v", err))
	}
	writer.Write(res)
}

// MakeHandlersMux makes a handler that dispatches for the various API endpoints.
func MakeHandlersMux(storeForRequest StoreForRequest, broker broker.BrokerSending, logger logger.Logger) *http.ServeMux {
	ctx := &context{
		storeForRequest: storeForRequest,
		broker:          broker,
		logger:          logger,
	}
	mux := http.NewServeMux()
	mux.Handle("/broadcast", &BroadcastHandler{context: ctx})
	mux.Handle("/notify", &UnicastHandler{context: ctx})
	mux.Handle("/register", &RegisterHandler{context: ctx})
	return mux
}
