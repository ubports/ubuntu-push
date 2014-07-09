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

package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/server/store"
	help "launchpad.net/ubuntu-push/testing"
)

func TestHandlers(t *testing.T) { TestingT(t) }

type handlersSuite struct {
	messageEndpoint string
	json            string
	client          *http.Client
	c               *C
	testlog         *help.TestLogger
}

var _ = Suite(&handlersSuite{})

func (s *handlersSuite) SetUpTest(c *C) {
	s.client = &http.Client{}
	s.testlog = help.NewTestLogger(c, "error")
}

func (s *handlersSuite) TestAPIError(c *C) {
	var apiErr error = &APIError{400, invalidRequest, "Message"}
	c.Check(apiErr.Error(), Equals, "api invalid-request: Message")
	wire, err := json.Marshal(apiErr)
	c.Assert(err, IsNil)
	c.Check(string(wire), Equals, `{"error":"invalid-request","message":"Message"}`)
}

func (s *handlersSuite) TestReadBodyReadError(c *C) {
	r := bytes.NewReader([]byte{}) // eof too early
	req, err := http.NewRequest("POST", "", r)
	c.Assert(err, IsNil)
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = 1000
	_, err = ReadBody(req, 2000)
	c.Check(err, Equals, ErrCouldNotReadBody)
}

func (s *handlersSuite) TestReadBodyTooBig(c *C) {
	r := bytes.NewReader([]byte{}) // not read
	req, err := http.NewRequest("POST", "", r)
	c.Assert(err, IsNil)
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = 3000
	_, err = ReadBody(req, 2000)
	c.Check(err, Equals, ErrRequestBodyTooLarge)
}

type testStoreAccess func(w http.ResponseWriter, request *http.Request) (store.PendingStore, error)

func (tsa testStoreAccess) StoreForRequest(w http.ResponseWriter, request *http.Request) (store.PendingStore, error) {
	return tsa(w, request)
}

func (tsa testStoreAccess) GetMaxNotificationsPerApplication() int {
	return 4
}

func (s *handlersSuite) TestGetStore(c *C) {
	ctx := &context{storage: testStoreAccess(func(w http.ResponseWriter, r *http.Request) (store.PendingStore, error) {
		return nil, ErrStoreUnavailable
	})}
	sto, apiErr := ctx.getStore(nil, nil)
	c.Check(sto, IsNil)
	c.Check(apiErr, Equals, ErrStoreUnavailable)

	ctx = &context{storage: testStoreAccess(func(w http.ResponseWriter, r *http.Request) (store.PendingStore, error) {
		return nil, errors.New("something else")
	}), logger: s.testlog}
	sto, apiErr = ctx.getStore(nil, nil)
	c.Check(sto, IsNil)
	c.Check(apiErr, Equals, ErrUnknown)
	c.Check(s.testlog.Captured(), Equals, "ERROR failed to get store: something else\n")
}

var future = time.Now().Add(4 * time.Hour).Format(time.RFC3339)

func (s *handlersSuite) TestCheckCastBroadcastAndCommon(c *C) {
	payload := json.RawMessage(`{"foo":"bar"}`)
	broadcast := &Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     payload,
	}
	expire, err := checkBroadcast(broadcast)
	c.Check(err, IsNil)
	c.Check(expire.Format(time.RFC3339), Equals, future)

	broadcast = &Broadcast{
		Channel:  "system",
		ExpireOn: future,
	}
	_, err = checkBroadcast(broadcast)
	c.Check(err, Equals, ErrMissingData)

	broadcast = &Broadcast{
		Channel:  "system",
		ExpireOn: "12:00",
		Data:     payload,
	}
	_, err = checkBroadcast(broadcast)
	c.Check(err, Equals, ErrInvalidExpiration)

	broadcast = &Broadcast{
		Channel:  "system",
		ExpireOn: time.Now().Add(-10 * time.Hour).Format(time.RFC3339),
		Data:     payload,
	}
	_, err = checkBroadcast(broadcast)
	c.Check(err, Equals, ErrPastExpiration)
}

type checkBrokerSending struct {
	store         store.PendingStore
	chanId        store.InternalChannelId
	err           error
	top           int64
	notifications []protocol.Notification
}

func (cbsend *checkBrokerSending) Broadcast(chanId store.InternalChannelId) {
	top, notifications, err := cbsend.store.GetChannelSnapshot(chanId)
	cbsend.err = err
	cbsend.chanId = chanId
	cbsend.top = top
	cbsend.notifications = notifications
}

func (cbsend *checkBrokerSending) Unicast(chanIds ...store.InternalChannelId) {
	// for now
	if len(chanIds) != 1 {
		panic("not expecting many chan ids for now")
	}
	cbsend.Broadcast(chanIds[0])
}

func (s *handlersSuite) TestDoBroadcast(c *C) {
	sto := store.NewInMemoryPendingStore()
	bsend := &checkBrokerSending{store: sto}
	ctx := &context{nil, bsend, nil}
	payload := json.RawMessage(`{"a": 1}`)
	res, apiErr := doBroadcast(ctx, sto, &Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     payload,
	})
	c.Assert(apiErr, IsNil)
	c.Assert(res, IsNil)
	c.Check(bsend.err, IsNil)
	c.Check(bsend.chanId, Equals, store.SystemInternalChannelId)
	c.Check(bsend.top, Equals, int64(1))
	c.Check(bsend.notifications, DeepEquals, help.Ns(payload))
}

func (s *handlersSuite) TestDoBroadcastUnknownChannel(c *C) {
	sto := store.NewInMemoryPendingStore()
	_, apiErr := doBroadcast(nil, sto, &Broadcast{
		Channel:  "unknown",
		ExpireOn: future,
		Data:     json.RawMessage(`{"a": 1}`),
	})
	c.Check(apiErr, Equals, ErrUnknownChannel)
}

type interceptInMemoryPendingStore struct {
	*store.InMemoryPendingStore
	intercept func(meth string, err error) error
}

func (isto *interceptInMemoryPendingStore) Register(appId, deviceId string) (string, error) {
	token, err := isto.InMemoryPendingStore.Register(appId, deviceId)
	return token, isto.intercept("Register", err)
}

func (isto *interceptInMemoryPendingStore) Unregister(appId, deviceId string) error {
	err := isto.InMemoryPendingStore.Unregister(appId, deviceId)
	return isto.intercept("Unregister", err)
}

func (isto *interceptInMemoryPendingStore) GetInternalChannelIdFromToken(token, appId, userId, deviceId string) (store.InternalChannelId, error) {
	chanId, err := isto.InMemoryPendingStore.GetInternalChannelIdFromToken(token, appId, userId, deviceId)
	return chanId, isto.intercept("GetInternalChannelIdFromToken", err)
}

func (isto *interceptInMemoryPendingStore) GetInternalChannelId(channel string) (store.InternalChannelId, error) {
	chanId, err := isto.InMemoryPendingStore.GetInternalChannelId(channel)
	return chanId, isto.intercept("GetInternalChannelId", err)
}

func (isto *interceptInMemoryPendingStore) AppendToChannel(chanId store.InternalChannelId, payload json.RawMessage, expiration time.Time) error {
	err := isto.InMemoryPendingStore.AppendToChannel(chanId, payload, expiration)
	return isto.intercept("AppendToChannel", err)
}

func (isto *interceptInMemoryPendingStore) AppendToUnicastChannel(chanId store.InternalChannelId, appId string, payload json.RawMessage, msgId string, expiration time.Time) error {
	err := isto.InMemoryPendingStore.AppendToUnicastChannel(chanId, appId, payload, msgId, expiration)
	return isto.intercept("AppendToUnicastChannel", err)
}

func (isto *interceptInMemoryPendingStore) GetChannelUnfiltered(chanId store.InternalChannelId) (int64, []protocol.Notification, []store.Metadata, error) {
	top, notifs, meta, err := isto.InMemoryPendingStore.GetChannelUnfiltered(chanId)
	return top, notifs, meta, isto.intercept("GetChannelUnfiltered", err)
}

func (isto *interceptInMemoryPendingStore) Scrub(chanId store.InternalChannelId, appId string) error {
	err := isto.InMemoryPendingStore.Scrub(chanId, appId)
	return isto.intercept("Scrub", err)
}

func (s *handlersSuite) TestDoBroadcastUnknownError(c *C) {
	sto := &interceptInMemoryPendingStore{
		store.NewInMemoryPendingStore(),
		func(meth string, err error) error {
			return errors.New("other")
		},
	}
	_, apiErr := doBroadcast(nil, sto, &Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     json.RawMessage(`{"a": 1}`),
	})
	c.Check(apiErr, Equals, ErrUnknown)
}

func (s *handlersSuite) TestDoBroadcastCouldNotStoreNotification(c *C) {
	sto := &interceptInMemoryPendingStore{
		store.NewInMemoryPendingStore(),
		func(meth string, err error) error {
			if meth == "AppendToChannel" {
				return errors.New("fail")
			}
			return err
		},
	}
	ctx := &context{logger: s.testlog}
	_, apiErr := doBroadcast(ctx, sto, &Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     json.RawMessage(`{"a": 1}`),
	})
	c.Check(apiErr, Equals, ErrCouldNotStoreNotification)
	c.Check(s.testlog.Captured(), Equals, "ERROR could not store notification: fail\n")
}

func (s *handlersSuite) TestCheckUnicast(c *C) {
	payload := json.RawMessage(`{"foo":"bar"}`)
	unicast := func() *Unicast {
		return &Unicast{
			UserId:   "user1",
			DeviceId: "DEV1",
			AppId:    "app1",
			ExpireOn: future,
			Data:     payload,
		}
	}
	u := unicast()
	expire, apiErr := checkUnicast(u)
	c.Assert(apiErr, IsNil)
	c.Check(expire.Format(time.RFC3339), Equals, future)

	u = unicast()
	u.UserId = ""
	u.DeviceId = ""
	u.Token = "TOKEN"
	expire, apiErr = checkUnicast(u)
	c.Assert(apiErr, IsNil)
	c.Check(expire.Format(time.RFC3339), Equals, future)

	u = unicast()
	u.UserId = ""
	expire, apiErr = checkUnicast(u)
	c.Check(apiErr, Equals, ErrMissingIdField)

	u = unicast()
	u.AppId = ""
	expire, apiErr = checkUnicast(u)
	c.Check(apiErr, Equals, ErrMissingIdField)

	u = unicast()
	u.DeviceId = ""
	expire, apiErr = checkUnicast(u)
	c.Check(apiErr, Equals, ErrMissingIdField)

	u = unicast()
	u.Data = json.RawMessage(nil)
	expire, apiErr = checkUnicast(u)
	c.Check(apiErr, Equals, ErrMissingData)
}

func (s *handlersSuite) TestGenerateMsgId(c *C) {
	msgId := generateMsgId()
	decoded, err := base64.StdEncoding.DecodeString(msgId)
	c.Assert(err, IsNil)
	c.Check(decoded, HasLen, 16)
}

func (s *handlersSuite) TestDoUnicast(c *C) {
	prevGenMsgId := generateMsgId
	defer func() {
		generateMsgId = prevGenMsgId
	}()
	generateMsgId = func() string {
		return "MSG-ID"
	}
	sto := store.NewInMemoryPendingStore()
	bsend := &checkBrokerSending{store: sto}
	ctx := &context{testStoreAccess(nil), bsend, nil}
	payload := json.RawMessage(`{"a": 1}`)
	res, apiErr := doUnicast(ctx, sto, &Unicast{
		UserId:   "user1",
		DeviceId: "DEV1",
		AppId:    "app1",
		ExpireOn: future,
		Data:     payload,
	})
	c.Assert(apiErr, IsNil)
	c.Check(res, IsNil)
	c.Check(bsend.err, IsNil)
	c.Check(bsend.chanId, Equals, store.UnicastInternalChannelId("user1", "DEV1"))
	c.Check(bsend.top, Equals, int64(0))
	c.Check(bsend.notifications, DeepEquals, []protocol.Notification{
		protocol.Notification{
			AppId:   "app1",
			MsgId:   "MSG-ID",
			Payload: payload,
		},
	})
}

func (s *handlersSuite) TestDoUnicastMissingIdField(c *C) {
	sto := store.NewInMemoryPendingStore()
	_, apiErr := doUnicast(nil, sto, &Unicast{
		ExpireOn: future,
		Data:     json.RawMessage(`{"a": 1}`),
	})
	c.Check(apiErr, Equals, ErrMissingIdField)
}

func (s *handlersSuite) TestDoUnicastCouldNotStoreNotification(c *C) {
	sto := &interceptInMemoryPendingStore{
		store.NewInMemoryPendingStore(),
		func(meth string, err error) error {
			if meth == "AppendToUnicastChannel" {
				return errors.New("fail")
			}
			return err
		},
	}
	ctx := &context{storage: testStoreAccess(nil), logger: s.testlog}
	_, apiErr := doUnicast(ctx, sto, &Unicast{
		UserId:   "user1",
		DeviceId: "DEV1",
		AppId:    "app1",
		ExpireOn: future,
		Data:     json.RawMessage(`{"a": 1}`),
	})
	c.Check(apiErr, Equals, ErrCouldNotStoreNotification)
	c.Check(s.testlog.Captured(), Equals, "ERROR could not store notification: fail\n")
}

func (s *handlersSuite) TestDoUnicastCouldNotPeekAtNotifications(c *C) {
	sto := &interceptInMemoryPendingStore{
		store.NewInMemoryPendingStore(),
		func(meth string, err error) error {
			if meth == "GetChannelUnfiltered" {
				return errors.New("fail")
			}
			return err
		},
	}
	ctx := &context{storage: testStoreAccess(nil), logger: s.testlog}
	_, apiErr := doUnicast(ctx, sto, &Unicast{
		UserId:   "user1",
		DeviceId: "DEV1",
		AppId:    "app1",
		ExpireOn: future,
		Data:     json.RawMessage(`{"a": 1}`),
	})
	c.Check(apiErr, Equals, ErrCouldNotStoreNotification)
	c.Check(s.testlog.Captured(), Equals, "ERROR could not peek at notifications: fail\n")
}

func (s *handlersSuite) TestDoUnicastTooManyNotifications(c *C) {
	sto := store.NewInMemoryPendingStore()
	chanId := store.UnicastInternalChannelId("user1", "DEV1")
	expire := time.Now().Add(4 * time.Hour)
	n := json.RawMessage("{}")
	sto.AppendToUnicastChannel(chanId, "app1", n, "m1", expire)
	sto.AppendToUnicastChannel(chanId, "app1", n, "m2", expire)
	sto.AppendToUnicastChannel(chanId, "app1", n, "m3", expire)
	sto.AppendToUnicastChannel(chanId, "app1", n, "m4", expire)

	ctx := &context{storage: testStoreAccess(nil), logger: s.testlog}
	_, apiErr := doUnicast(ctx, sto, &Unicast{
		UserId:   "user1",
		DeviceId: "DEV1",
		AppId:    "app1",
		ExpireOn: future,
		Data:     json.RawMessage(`{"a": 1}`),
	})
	c.Check(apiErr, Equals, ErrTooManyPendingNotifications)
	c.Check(s.testlog.Captured(), Equals, "")
}

func (s *handlersSuite) TestDoUnicastWithScrub(c *C) {
	prevGenMsgId := generateMsgId
	defer func() {
		generateMsgId = prevGenMsgId
	}()
	generateMsgId = func() string {
		return "MSG-ID"
	}
	sto := store.NewInMemoryPendingStore()
	chanId := store.UnicastInternalChannelId("user1", "DEV1")
	expire := time.Now().Add(4 * time.Hour)
	old := time.Now().Add(-1 * time.Hour)
	n := json.RawMessage("{}")
	sto.AppendToUnicastChannel(chanId, "app1", n, "m1", expire)
	sto.AppendToUnicastChannel(chanId, "app1", n, "m2", old)
	sto.AppendToUnicastChannel(chanId, "app1", n, "m3", old)
	sto.AppendToUnicastChannel(chanId, "app1", n, "m4", expire)

	bsend := &checkBrokerSending{store: sto}
	ctx := &context{testStoreAccess(nil), bsend, nil}
	payload := json.RawMessage(`{"a": 1}`)
	res, apiErr := doUnicast(ctx, sto, &Unicast{
		UserId:   "user1",
		DeviceId: "DEV1",
		AppId:    "app1",
		ExpireOn: future,
		Data:     payload,
	})
	c.Assert(apiErr, IsNil)
	c.Check(res, IsNil)
	c.Check(bsend.err, IsNil)
	c.Check(bsend.chanId, Equals, store.UnicastInternalChannelId("user1", "DEV1"))
	c.Check(bsend.top, Equals, int64(0))
	c.Check(bsend.notifications, HasLen, 3)
	c.Check(bsend.notifications[0].MsgId, Equals, "m1")
	c.Check(bsend.notifications[1].MsgId, Equals, "m4")
	c.Check(bsend.notifications[2], DeepEquals, protocol.Notification{
		AppId:   "app1",
		MsgId:   "MSG-ID",
		Payload: payload,
	})
}

func (s *handlersSuite) TestDoUnicastWithScrubError(c *C) {
	sto := &interceptInMemoryPendingStore{
		store.NewInMemoryPendingStore(),
		func(meth string, err error) error {
			if meth == "Scrub" {
				return errors.New("fail")
			}
			return err
		},
	}
	chanId := store.UnicastInternalChannelId("user1", "DEV1")
	expire := time.Now().Add(4 * time.Hour)
	old := time.Now().Add(-1 * time.Hour)
	n := json.RawMessage("{}")
	sto.AppendToUnicastChannel(chanId, "app1", n, "m1", expire)
	sto.AppendToUnicastChannel(chanId, "app1", n, "m2", old)
	sto.AppendToUnicastChannel(chanId, "app1", n, "m3", old)
	sto.AppendToUnicastChannel(chanId, "app1", n, "m4", expire)

	ctx := &context{testStoreAccess(nil), nil, s.testlog}
	payload := json.RawMessage(`{"a": 1}`)
	_, apiErr := doUnicast(ctx, sto, &Unicast{
		UserId:   "user1",
		DeviceId: "DEV1",
		AppId:    "app1",
		ExpireOn: future,
		Data:     payload,
	})
	c.Check(apiErr, Equals, ErrCouldNotStoreNotification)
	c.Check(s.testlog.Captured(), Equals, "ERROR could not scrub channel: fail\n")
}

func (s *handlersSuite) TestDoUnicastCleanPending(c *C) {
	prevGenMsgId := generateMsgId
	defer func() {
		generateMsgId = prevGenMsgId
	}()
	generateMsgId = func() string {
		return "MSG-ID"
	}
	sto := store.NewInMemoryPendingStore()
	chanId := store.UnicastInternalChannelId("user1", "DEV1")
	expire := time.Now().Add(4 * time.Hour)
	n := json.RawMessage("{}")
	sto.AppendToUnicastChannel(chanId, "app1", n, "m1", expire)
	sto.AppendToUnicastChannel(chanId, "app1", n, "m2", expire)
	sto.AppendToUnicastChannel(chanId, "app1", n, "m3", expire)
	sto.AppendToUnicastChannel(chanId, "app1", n, "m4", expire)

	bsend := &checkBrokerSending{store: sto}
	ctx := &context{testStoreAccess(nil), bsend, nil}
	payload := json.RawMessage(`{"a": 1}`)
	res, apiErr := doUnicast(ctx, sto, &Unicast{
		UserId:       "user1",
		DeviceId:     "DEV1",
		AppId:        "app1",
		ExpireOn:     future,
		Data:         payload,
		CleanPending: true,
	})
	c.Assert(apiErr, IsNil)
	c.Check(res, IsNil)
	c.Check(bsend.err, IsNil)
	c.Check(bsend.chanId, Equals, store.UnicastInternalChannelId("user1", "DEV1"))
	c.Check(bsend.top, Equals, int64(0))
	c.Check(bsend.notifications, HasLen, 1)
	c.Check(bsend.notifications[0], DeepEquals, protocol.Notification{
		AppId:   "app1",
		MsgId:   "MSG-ID",
		Payload: payload,
	})
}

func (s *handlersSuite) TestDoUnicastFromTokenFailures(c *C) {
	fail := errors.New("fail")
	sto := &interceptInMemoryPendingStore{
		store.NewInMemoryPendingStore(),
		func(meth string, err error) error {
			if meth == "GetInternalChannelIdFromToken" {
				return fail
			}
			return err
		},
	}
	ctx := &context{logger: s.testlog}
	u := &Unicast{
		Token:    "tok",
		AppId:    "app1",
		ExpireOn: future,
		Data:     json.RawMessage(`{"a": 1}`),
	}
	_, apiErr := doUnicast(ctx, sto, u)
	c.Check(apiErr, Equals, ErrCouldNotResolveToken)
	c.Check(s.testlog.Captured(), Equals, "ERROR could not resolve token: fail\n")
	s.testlog.ResetCapture()

	fail = store.ErrUnknownToken
	_, apiErr = doUnicast(ctx, sto, u)
	c.Check(apiErr, Equals, ErrUnknownToken)
	c.Check(s.testlog.Captured(), Equals, "")
	fail = store.ErrUnauthorized
	_, apiErr = doUnicast(ctx, sto, u)
	c.Check(apiErr, Equals, ErrUnauthorized)
	c.Check(s.testlog.Captured(), Equals, "")
}

func newPostRequest(path string, message interface{}, server *httptest.Server) *http.Request {
	packedMessage, err := json.Marshal(message)
	if err != nil {
		panic(err)
	}
	reader := bytes.NewReader(packedMessage)

	url := server.URL + path
	request, _ := http.NewRequest("POST", url, reader)
	request.ContentLength = int64(reader.Len())
	request.Header.Set("Content-Type", "application/json")

	return request
}

func getResponseBody(response *http.Response) ([]byte, error) {
	defer response.Body.Close()
	return ioutil.ReadAll(response.Body)
}

func checkError(c *C, response *http.Response, apiErr *APIError) {
	c.Check(response.StatusCode, Equals, apiErr.StatusCode)
	c.Check(response.Header.Get("Content-Type"), Equals, "application/json")
	error := &APIError{StatusCode: response.StatusCode}
	body, err := getResponseBody(response)
	c.Assert(err, IsNil)
	err = json.Unmarshal(body, error)
	c.Assert(err, IsNil)
	c.Check(error, DeepEquals, apiErr)
}

type testBrokerSending struct {
	chanId chan store.InternalChannelId
}

func (bsend testBrokerSending) Broadcast(chanId store.InternalChannelId) {
	bsend.chanId <- chanId
}

func (bsend testBrokerSending) Unicast(chanIds ...store.InternalChannelId) {
	// for now
	if len(chanIds) != 1 {
		panic("not expecting many chan ids for now")
	}
	bsend.chanId <- chanIds[0]
}

func (s *handlersSuite) TestRespondsToBasicSystemBroadcast(c *C) {
	sto := store.NewInMemoryPendingStore()
	storage := testStoreAccess(func(http.ResponseWriter, *http.Request) (store.PendingStore, error) {
		return sto, nil
	})
	bsend := testBrokerSending{make(chan store.InternalChannelId, 1)}
	testServer := httptest.NewServer(MakeHandlersMux(storage, bsend, nil))
	defer testServer.Close()

	payload := json.RawMessage(`{"foo":"bar"}`)

	request := newPostRequest("/broadcast", &Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     payload,
	}, testServer)

	response, err := s.client.Do(request)
	c.Assert(err, IsNil)

	c.Check(response.StatusCode, Equals, http.StatusOK)
	c.Check(response.Header.Get("Content-Type"), Equals, "application/json")
	body, err := getResponseBody(response)
	c.Assert(err, IsNil)
	dest := make(map[string]bool)
	err = json.Unmarshal(body, &dest)
	c.Assert(err, IsNil)
	c.Assert(dest, DeepEquals, map[string]bool{"ok": true})

	top, _, err := sto.GetChannelSnapshot(store.SystemInternalChannelId)
	c.Assert(err, IsNil)
	c.Check(top, Equals, int64(1))
	c.Check(<-bsend.chanId, Equals, store.SystemInternalChannelId)
}

func (s *handlersSuite) TestStoreUnavailable(c *C) {
	storage := testStoreAccess(func(http.ResponseWriter, *http.Request) (store.PendingStore, error) {
		return nil, ErrStoreUnavailable
	})
	testServer := httptest.NewServer(MakeHandlersMux(storage, nil, nil))
	defer testServer.Close()

	payload := json.RawMessage(`{"foo":"bar"}`)

	request := newPostRequest("/broadcast", &Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     payload,
	}, testServer)

	response, err := s.client.Do(request)
	c.Assert(err, IsNil)
	checkError(c, response, ErrStoreUnavailable)
}

func (s *handlersSuite) TestFromBroadcastError(c *C) {
	sto := store.NewInMemoryPendingStore()
	storage := testStoreAccess(func(http.ResponseWriter, *http.Request) (store.PendingStore, error) {
		return sto, nil
	})
	testServer := httptest.NewServer(MakeHandlersMux(storage, nil, nil))
	defer testServer.Close()

	payload := json.RawMessage(`{"foo":"bar"}`)

	request := newPostRequest("/broadcast", &Broadcast{
		Channel:  "unknown",
		ExpireOn: future,
		Data:     payload,
	}, testServer)

	response, err := s.client.Do(request)
	c.Assert(err, IsNil)
	checkError(c, response, ErrUnknownChannel)
}

func (s *handlersSuite) TestMissingData(c *C) {
	storage := testStoreAccess(func(http.ResponseWriter, *http.Request) (store.PendingStore, error) {
		return store.NewInMemoryPendingStore(), nil
	})
	ctx := &context{storage, nil, nil}
	testServer := httptest.NewServer(&JSONPostHandler{
		context:        ctx,
		parsingBodyObj: func() interface{} { return &Broadcast{} },
		doHandle:       doBroadcast,
	})
	defer testServer.Close()

	packedMessage := []byte(`{"channel": "system"}`)
	reader := bytes.NewReader(packedMessage)

	request, err := http.NewRequest("POST", testServer.URL, reader)
	c.Assert(err, IsNil)
	request.ContentLength = int64(len(packedMessage))
	request.Header.Set("Content-Type", "application/json")

	response, err := s.client.Do(request)
	c.Assert(err, IsNil)
	checkError(c, response, ErrMissingData)
}

func (s *handlersSuite) TestCannotBroadcastMalformedData(c *C) {
	storage := testStoreAccess(func(http.ResponseWriter, *http.Request) (store.PendingStore, error) {
		return store.NewInMemoryPendingStore(), nil
	})
	ctx := &context{storage, nil, nil}
	testServer := httptest.NewServer(&JSONPostHandler{
		context:        ctx,
		parsingBodyObj: func() interface{} { return &Broadcast{} },
	})
	defer testServer.Close()

	packedMessage := []byte("{some bogus-message: ")
	reader := bytes.NewReader(packedMessage)

	request, err := http.NewRequest("POST", testServer.URL, reader)
	c.Assert(err, IsNil)
	request.ContentLength = int64(len(packedMessage))
	request.Header.Set("Content-Type", "application/json")

	response, err := s.client.Do(request)
	c.Assert(err, IsNil)
	checkError(c, response, ErrMalformedJSONObject)
}

func (s *handlersSuite) TestCannotBroadcastTooBigMessages(c *C) {
	testServer := httptest.NewServer(&JSONPostHandler{})
	defer testServer.Close()

	bigString := strings.Repeat("a", MaxRequestBodyBytes)
	dataString := fmt.Sprintf(`"%v"`, bigString)

	request := newPostRequest("/", &Broadcast{
		Channel:  "some-channel",
		ExpireOn: future,
		Data:     json.RawMessage([]byte(dataString)),
	}, testServer)

	response, err := s.client.Do(request)
	c.Assert(err, IsNil)
	checkError(c, response, ErrRequestBodyTooLarge)
}

func (s *handlersSuite) TestCannotBroadcastWithoutContentLength(c *C) {
	testServer := httptest.NewServer(&JSONPostHandler{})
	defer testServer.Close()

	dataString := `{"foo":"bar"}`

	request := newPostRequest("/", &Broadcast{
		Channel:  "some-channel",
		ExpireOn: future,
		Data:     json.RawMessage([]byte(dataString)),
	}, testServer)
	request.ContentLength = -1

	response, err := s.client.Do(request)
	c.Assert(err, IsNil)
	checkError(c, response, ErrNoContentLengthProvided)
}

func (s *handlersSuite) TestCannotBroadcastEmptyMessages(c *C) {
	testServer := httptest.NewServer(&JSONPostHandler{})
	defer testServer.Close()

	packedMessage := make([]byte, 0)
	reader := bytes.NewReader(packedMessage)

	request, err := http.NewRequest("POST", testServer.URL, reader)
	c.Assert(err, IsNil)
	request.ContentLength = int64(len(packedMessage))
	request.Header.Set("Content-Type", "application/json")

	response, err := s.client.Do(request)
	c.Assert(err, IsNil)
	checkError(c, response, ErrRequestBodyEmpty)
}

func (s *handlersSuite) TestCannotBroadcastNonJSONMessages(c *C) {
	testServer := httptest.NewServer(&JSONPostHandler{})
	defer testServer.Close()

	dataString := `{"foo":"bar"}`

	request := newPostRequest("/", &Broadcast{
		Channel:  "some-channel",
		ExpireOn: future,
		Data:     json.RawMessage([]byte(dataString)),
	}, testServer)
	request.Header.Set("Content-Type", "text/plain")

	response, err := s.client.Do(request)
	c.Assert(err, IsNil)
	checkError(c, response, ErrWrongContentType)
}

func (s *handlersSuite) TestCannotBroadcastNonPostMessages(c *C) {
	testServer := httptest.NewServer(&JSONPostHandler{})
	defer testServer.Close()

	dataString := `{"foo":"bar"}`
	packedMessage, err := json.Marshal(&Broadcast{
		Channel:  "some-channel",
		ExpireOn: future,
		Data:     json.RawMessage([]byte(dataString)),
	})
	s.c.Assert(err, IsNil)
	reader := bytes.NewReader(packedMessage)

	request, err := http.NewRequest("GET", testServer.URL, reader)
	c.Assert(err, IsNil)
	request.ContentLength = int64(len(packedMessage))
	request.Header.Set("Content-Type", "application/json")

	response, err := s.client.Do(request)
	c.Assert(err, IsNil)

	checkError(c, response, ErrWrongRequestMethod)
}

const OK = `.*"ok":true.*`

func (s *handlersSuite) TestRespondsUnicast(c *C) {
	sto := store.NewInMemoryPendingStore()
	storage := testStoreAccess(func(http.ResponseWriter, *http.Request) (store.PendingStore, error) {
		return sto, nil
	})
	bsend := testBrokerSending{make(chan store.InternalChannelId, 1)}
	testServer := httptest.NewServer(MakeHandlersMux(storage, bsend, nil))
	defer testServer.Close()

	payload := json.RawMessage(`{"foo":"bar"}`)

	request := newPostRequest("/notify", &Unicast{
		UserId:   "user2",
		DeviceId: "dev3",
		AppId:    "app2",
		ExpireOn: future,
		Data:     payload,
	}, testServer)

	response, err := s.client.Do(request)
	c.Assert(err, IsNil)

	c.Check(response.StatusCode, Equals, http.StatusOK)
	c.Check(response.Header.Get("Content-Type"), Equals, "application/json")
	body, err := getResponseBody(response)
	c.Assert(err, IsNil)
	c.Assert(string(body), Matches, OK)

	chanId := store.UnicastInternalChannelId("user2", "dev3")
	c.Check(<-bsend.chanId, Equals, chanId)
	top, notifications, err := sto.GetChannelSnapshot(chanId)
	c.Assert(err, IsNil)
	c.Check(top, Equals, int64(0))
	c.Check(notifications, HasLen, 1)
}

func (s *handlersSuite) TestCheckRegister(c *C) {
	registration := func() *Registration {
		return &Registration{
			DeviceId: "DEV1",
			AppId:    "app1",
		}
	}
	reg := registration()
	apiErr := checkRegister(reg)
	c.Assert(apiErr, IsNil)

	reg = registration()
	reg.AppId = ""
	apiErr = checkRegister(reg)
	c.Check(apiErr, Equals, ErrMissingIdField)

	reg = registration()
	reg.DeviceId = ""
	apiErr = checkRegister(reg)
	c.Check(apiErr, Equals, ErrMissingIdField)
}

func (s *handlersSuite) TestDoRegisterMissingIdField(c *C) {
	sto := store.NewInMemoryPendingStore()
	token, apiErr := doRegister(nil, sto, &Registration{})
	c.Check(apiErr, Equals, ErrMissingIdField)
	c.Check(token, IsNil)
}

func (s *handlersSuite) TestDoRegisterCouldNotMakeToken(c *C) {
	sto := &interceptInMemoryPendingStore{
		store.NewInMemoryPendingStore(),
		func(meth string, err error) error {
			if meth == "Register" {
				return errors.New("fail")
			}
			return err
		},
	}
	ctx := &context{logger: s.testlog}
	_, apiErr := doRegister(ctx, sto, &Registration{
		DeviceId: "DEV1",
		AppId:    "app1",
	})
	c.Check(apiErr, Equals, ErrCouldNotMakeToken)
	c.Check(s.testlog.Captured(), Equals, "ERROR could not make a token: fail\n")
}

func (s *handlersSuite) TestRespondsToRegisterAndUnicast(c *C) {
	sto := store.NewInMemoryPendingStore()
	storage := testStoreAccess(func(http.ResponseWriter, *http.Request) (store.PendingStore, error) {
		return sto, nil
	})
	bsend := testBrokerSending{make(chan store.InternalChannelId, 1)}
	testServer := httptest.NewServer(MakeHandlersMux(storage, bsend, nil))
	defer testServer.Close()

	request := newPostRequest("/register", &Registration{
		DeviceId: "dev3",
		AppId:    "app2",
	}, testServer)

	response, err := s.client.Do(request)
	c.Assert(err, IsNil)

	c.Check(response.StatusCode, Equals, http.StatusOK)
	c.Check(response.Header.Get("Content-Type"), Equals, "application/json")
	body, err := getResponseBody(response)
	c.Assert(err, IsNil)
	c.Assert(string(body), Matches, OK)
	var reg map[string]interface{}
	err = json.Unmarshal(body, &reg)
	c.Assert(err, IsNil)

	token, ok := reg["token"].(string)
	c.Assert(ok, Equals, true)
	c.Check(token, Not(Equals), nil)

	payload := json.RawMessage(`{"foo":"bar"}`)

	request = newPostRequest("/notify", &Unicast{
		Token:    token,
		AppId:    "app2",
		ExpireOn: future,
		Data:     payload,
	}, testServer)

	response, err = s.client.Do(request)
	c.Assert(err, IsNil)

	c.Check(response.StatusCode, Equals, http.StatusOK)
	c.Check(response.Header.Get("Content-Type"), Equals, "application/json")
	body, err = getResponseBody(response)
	c.Assert(err, IsNil)
	c.Assert(string(body), Matches, OK)

	chanId := store.UnicastInternalChannelId("dev3", "dev3")
	c.Check(<-bsend.chanId, Equals, chanId)
	top, notifications, err := sto.GetChannelSnapshot(chanId)
	c.Assert(err, IsNil)
	c.Check(top, Equals, int64(0))
	c.Check(notifications, HasLen, 1)
}

func (s *handlersSuite) TestRespondsToUnregister(c *C) {
	yay := make(chan bool, 1)
	sto := &interceptInMemoryPendingStore{
		store.NewInMemoryPendingStore(),
		func(meth string, err error) error {
			if meth == "Unregister" {
				yay <- true
			}
			return err
		},
	}
	storage := testStoreAccess(func(http.ResponseWriter, *http.Request) (store.PendingStore, error) {
		return sto, nil
	})
	bsend := testBrokerSending{make(chan store.InternalChannelId, 1)}
	testServer := httptest.NewServer(MakeHandlersMux(storage, bsend, nil))
	defer testServer.Close()

	request := newPostRequest("/unregister", &Registration{
		DeviceId: "dev3",
		AppId:    "app2",
	}, testServer)

	response, err := s.client.Do(request)
	c.Assert(err, IsNil)

	c.Check(response.StatusCode, Equals, http.StatusOK)
	c.Check(response.Header.Get("Content-Type"), Equals, "application/json")
	body, err := getResponseBody(response)
	c.Assert(err, IsNil)
	c.Assert(string(body), Matches, OK)
	c.Check(yay, HasLen, 1)
}

func (s *handlersSuite) TestDoUnregisterMissingIdField(c *C) {
	sto := store.NewInMemoryPendingStore()
	token, apiErr := doUnregister(nil, sto, &Registration{})
	c.Check(apiErr, Equals, ErrMissingIdField)
	c.Check(token, IsNil)
}

func (s *handlersSuite) TestDoUnregisterCouldNotRemoveToken(c *C) {
	sto := &interceptInMemoryPendingStore{
		store.NewInMemoryPendingStore(),
		func(meth string, err error) error {
			if meth == "Unregister" {
				return errors.New("fail")
			}
			return err
		},
	}
	ctx := &context{logger: s.testlog}
	_, apiErr := doUnregister(ctx, sto, &Registration{
		DeviceId: "DEV1",
		AppId:    "app1",
	})
	c.Check(apiErr, Equals, ErrCouldNotRemoveToken)
	c.Check(s.testlog.Captured(), Equals, "ERROR could not remove token: fail\n")
}
