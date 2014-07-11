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
	"launchpad.net/ubuntu-push/protocol"
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
	// extra information
	Extra json.RawMessage `json:"extra,omitempty"`
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
	tooManyPending = "too-many-pending"
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
		nil,
	}
	ErrRequestBodyEmpty = &APIError{
		http.StatusBadRequest,
		invalidRequest,
		"Request body empty",
		nil,
	}
	ErrRequestBodyTooLarge = &APIError{
		http.StatusRequestEntityTooLarge,
		invalidRequest,
		"Request body too large",
		nil,
	}
	ErrWrongContentType = &APIError{
		http.StatusUnsupportedMediaType,
		invalidRequest,
		"Wrong content type, should be application/json",
		nil,
	}
	ErrWrongRequestMethod = &APIError{
		http.StatusMethodNotAllowed,
		invalidRequest,
		"Wrong request method, should be POST",
		nil,
	}
	ErrMalformedJSONObject = &APIError{
		http.StatusBadRequest,
		invalidRequest,
		"Malformed JSON Object",
		nil,
	}
	ErrCouldNotReadBody = &APIError{
		http.StatusBadRequest,
		ioError,
		"Could not read request body",
		nil,
	}
	ErrMissingIdField = &APIError{
		http.StatusBadRequest,
		invalidRequest,
		"Missing id field",
		nil,
	}
	ErrMissingData = &APIError{
		http.StatusBadRequest,
		invalidRequest,
		"Missing data field",
		nil,
	}
	ErrInvalidExpiration = &APIError{
		http.StatusBadRequest,
		invalidRequest,
		"Invalid expiration date",
		nil,
	}
	ErrPastExpiration = &APIError{
		http.StatusBadRequest,
		invalidRequest,
		"Past expiration date",
		nil,
	}
	ErrUnknownChannel = &APIError{
		http.StatusBadRequest,
		unknownChannel,
		"Unknown channel",
		nil,
	}
	ErrUnknownToken = &APIError{
		http.StatusBadRequest,
		unknownToken,
		"Unknown token",
		nil,
	}
	ErrUnknown = &APIError{
		http.StatusInternalServerError,
		internalError,
		"Unknown error",
		nil,
	}
	ErrStoreUnavailable = &APIError{
		http.StatusServiceUnavailable,
		unavailable,
		"Message store unavailable",
		nil,
	}
	ErrCouldNotStoreNotification = &APIError{
		http.StatusServiceUnavailable,
		unavailable,
		"Could not store notification",
		nil,
	}
	ErrCouldNotMakeToken = &APIError{
		http.StatusServiceUnavailable,
		unavailable,
		"Could not make token",
		nil,
	}
	ErrCouldNotRemoveToken = &APIError{
		http.StatusServiceUnavailable,
		unavailable,
		"Could not remove token",
		nil,
	}
	ErrCouldNotResolveToken = &APIError{
		http.StatusServiceUnavailable,
		unavailable,
		"Could not resolve token",
		nil,
	}
	ErrUnauthorized = &APIError{
		http.StatusUnauthorized,
		unauthorized,
		"Unauthorized",
		nil,
	}
	ErrTooManyPendingNotifications = &APIError{
		http.StatusRequestEntityTooLarge,
		tooManyPending,
		"Too many pending notifications for this application",
		nil,
	}
)

func apiErrorWithExtra(apiErr *APIError, extra interface{}) *APIError {
	var clone APIError = *apiErr
	b, err := json.Marshal(extra)
	if err != nil {
		panic(fmt.Errorf("couldn't marshal our own errors: %v", err))
	}
	clone.Extra = json.RawMessage(b)
	return &clone
}

type Registration struct {
	DeviceId string `json:"deviceid"`
	AppId    string `json:"appid"`
}

type Unicast struct {
	Token    string          `json:"token"`
	UserId   string          `json:"userid"`   // not part of the official API
	DeviceId string          `json:"deviceid"` // not part of the official API
	AppId    string          `json:"appid"`
	ExpireOn string          `json:"expire_on"`
	Data     json.RawMessage `json:"data"`
	// clear all pending messages for appid
	ClearPending bool `json:"clear_pending,omitempty"`
	// replace pending messages with the same replace_tag
	ReplaceTag string `json:"replace_tag,omitempty"`
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

// StoreAccess lets get a notification pending store and parameters
// for storage.
type StoreAccess interface {
	// StoreForRequest gets a pending store for the request.
	StoreForRequest(w http.ResponseWriter, request *http.Request) (store.PendingStore, error)
	// GetMaxNotificationsPerApplication gets the maximum number
	// of pending notifications allowed for a signle application.
	GetMaxNotificationsPerApplication() int
}

// context holds the interfaces to delegate to serving requests
type context struct {
	storage StoreAccess
	broker  broker.BrokerSending
	logger  logger.Logger
}

func (ctx *context) getStore(w http.ResponseWriter, request *http.Request) (store.PendingStore, *APIError) {
	sto, err := ctx.storage.StoreForRequest(w, request)
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

// JSONPostHandler is able to handle POST requests with a JSON body
// delegating for the actual details.
type JSONPostHandler struct {
	*context
	parsingBodyObj func() interface{}
	doHandle       func(ctx *context, sto store.PendingStore, parsedBodyObj interface{}) (map[string]interface{}, *APIError)
}

func (h *JSONPostHandler) prepare(w http.ResponseWriter, request *http.Request) (interface{}, store.PendingStore, *APIError) {
	body, apiErr := ReadBody(request, MaxRequestBodyBytes)
	if apiErr != nil {
		return nil, nil, apiErr
	}

	parsedBodyObj := h.parsingBodyObj()
	err := json.Unmarshal(body, parsedBodyObj)
	if err != nil {
		return nil, nil, ErrMalformedJSONObject
	}

	sto, apiErr := h.getStore(w, request)
	if apiErr != nil {
		return nil, nil, apiErr
	}
	return parsedBodyObj, sto, nil
}

func (h *JSONPostHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var apiErr *APIError
	defer func() {
		if apiErr != nil {
			RespondError(writer, apiErr)
		}
	}()

	parsedBodyObj, sto, apiErr := h.prepare(writer, request)
	if apiErr != nil {
		return
	}
	defer sto.Close()

	res, apiErr := h.doHandle(h.context, sto, parsedBodyObj)
	if apiErr != nil {
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	if res == nil {
		fmt.Fprintf(writer, `{"ok":true}`)
	} else {
		res["ok"] = true
		resp, err := json.Marshal(res)
		if err != nil {
			panic(fmt.Errorf("couldn't marshal our own response: %v", err))
		}
		writer.Write(resp)
	}
}

func doBroadcast(ctx *context, sto store.PendingStore, parsedBodyObj interface{}) (map[string]interface{}, *APIError) {
	bcast := parsedBodyObj.(*Broadcast)
	expire, apiErr := checkBroadcast(bcast)
	if apiErr != nil {
		return nil, apiErr
	}
	chanId, err := sto.GetInternalChannelId(bcast.Channel)
	if err != nil {
		switch err {
		case store.ErrUnknownChannel:
			return nil, ErrUnknownChannel
		default:
			return nil, ErrUnknown
		}
	}
	err = sto.AppendToChannel(chanId, bcast.Data, expire)
	if err != nil {
		ctx.logger.Errorf("could not store notification: %v", err)
		return nil, ErrCouldNotStoreNotification
	}

	ctx.broker.Broadcast(chanId)
	return nil, nil
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

func doUnicast(ctx *context, sto store.PendingStore, parsedBodyObj interface{}) (map[string]interface{}, *APIError) {
	ucast := parsedBodyObj.(*Unicast)
	expire, apiErr := checkUnicast(ucast)
	if apiErr != nil {
		return nil, apiErr
	}
	chanId, err := sto.GetInternalChannelIdFromToken(ucast.Token, ucast.AppId, ucast.UserId, ucast.DeviceId)
	if err != nil {
		switch err {
		case store.ErrUnknownToken:
			return nil, ErrUnknownToken
		case store.ErrUnauthorized:
			return nil, ErrUnauthorized
		default:
			ctx.logger.Errorf("could not resolve token: %v", err)
			return nil, ErrCouldNotResolveToken
		}
	}

	_, notifs, meta, err := sto.GetChannelUnfiltered(chanId)
	if err != nil {
		ctx.logger.Errorf("could not peek at notifications: %v", err)
		return nil, ErrCouldNotStoreNotification
	}
	expired := 0
	replaceable := 0
	forApp := 0
	replaceTag := ucast.ReplaceTag
	scrubCriteria := []string(nil)
	now := time.Now()
	var last *protocol.Notification
	for i, notif := range notifs {
		if meta[i].Before(now) {
			expired++
			continue
		}
		if notif.AppId == ucast.AppId {
			if replaceTag != "" && replaceTag == meta[i].ReplaceTag {
				// this we will scrub
				replaceable++
				continue
			}
			forApp++
		}
		last = &notif
	}
	if ucast.ClearPending {
		scrubCriteria = []string{ucast.AppId}
	} else if forApp >= ctx.storage.GetMaxNotificationsPerApplication() {
		return nil, apiErrorWithExtra(ErrTooManyPendingNotifications,
			&last.Payload)
	} else if replaceable > 0 {
		scrubCriteria = []string{ucast.AppId, replaceTag}
	}
	if expired > 0 || scrubCriteria != nil {
		err := sto.Scrub(chanId, scrubCriteria...)
		if err != nil {
			ctx.logger.Errorf("could not scrub channel: %v", err)
			return nil, ErrCouldNotStoreNotification
		}
	}

	msgId := generateMsgId()

	meta1 := store.Metadata{
		Expiration: expire,
		ReplaceTag: ucast.ReplaceTag,
	}

	err = sto.AppendToUnicastChannel(chanId, ucast.AppId, ucast.Data, msgId, meta1)
	if err != nil {
		ctx.logger.Errorf("could not store notification: %v", err)
		return nil, ErrCouldNotStoreNotification
	}

	ctx.broker.Unicast(chanId)
	return nil, nil
}

func checkRegister(reg *Registration) *APIError {
	if reg.DeviceId == "" || reg.AppId == "" {
		return ErrMissingIdField
	}
	return nil
}

func doRegister(ctx *context, sto store.PendingStore, parsedBodyObj interface{}) (map[string]interface{}, *APIError) {
	reg := parsedBodyObj.(*Registration)
	apiErr := checkRegister(reg)
	if apiErr != nil {
		return nil, apiErr
	}
	token, err := sto.Register(reg.DeviceId, reg.AppId)
	if err != nil {
		ctx.logger.Errorf("could not make a token: %v", err)
		return nil, ErrCouldNotMakeToken
	}
	return map[string]interface{}{"token": token}, nil
}

func doUnregister(ctx *context, sto store.PendingStore, parsedBodyObj interface{}) (map[string]interface{}, *APIError) {
	reg := parsedBodyObj.(*Registration)
	apiErr := checkRegister(reg)
	if apiErr != nil {
		return nil, apiErr
	}
	err := sto.Unregister(reg.DeviceId, reg.AppId)
	if err != nil {
		ctx.logger.Errorf("could not remove token: %v", err)
		return nil, ErrCouldNotRemoveToken
	}
	return nil, nil
}

// MakeHandlersMux makes a handler that dispatches for the various API endpoints.
func MakeHandlersMux(storage StoreAccess, broker broker.BrokerSending, logger logger.Logger) *http.ServeMux {
	ctx := &context{
		storage: storage,
		broker:  broker,
		logger:  logger,
	}
	mux := http.NewServeMux()
	mux.Handle("/broadcast", &JSONPostHandler{
		context:        ctx,
		parsingBodyObj: func() interface{} { return &Broadcast{} },
		doHandle:       doBroadcast,
	})
	mux.Handle("/notify", &JSONPostHandler{
		context:        ctx,
		parsingBodyObj: func() interface{} { return &Unicast{} },
		doHandle:       doUnicast,
	})
	mux.Handle("/register", &JSONPostHandler{
		context:        ctx,
		parsingBodyObj: func() interface{} { return &Registration{} },
		doHandle:       doRegister,
	})
	mux.Handle("/unregister", &JSONPostHandler{
		context:        ctx,
		parsingBodyObj: func() interface{} { return &Registration{} },
		doHandle:       doUnregister,
	})
	return mux
}
