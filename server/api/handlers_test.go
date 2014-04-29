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
	helpers "launchpad.net/ubuntu-push/testing"
)

func TestHandlers(t *testing.T) { TestingT(t) }

type handlersSuite struct {
	messageEndpoint string
	json            string
	client          *http.Client
	c               *C
	testlog         *helpers.TestLogger
}

var _ = Suite(&handlersSuite{})

func (s *handlersSuite) SetUpTest(c *C) {
	s.client = &http.Client{}
	s.testlog = helpers.NewTestLogger(c, "error")
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

func (s *handlersSuite) TestGetStore(c *C) {
	ctx := &context{storeForRequest: func(w http.ResponseWriter, r *http.Request) (store.PendingStore, error) {
		return nil, ErrStoreUnavailable
	}}
	sto, apiErr := ctx.getStore(nil, nil)
	c.Check(sto, IsNil)
	c.Check(apiErr, Equals, ErrStoreUnavailable)

	ctx = &context{storeForRequest: func(w http.ResponseWriter, r *http.Request) (store.PendingStore, error) {
		return nil, errors.New("something else")
	}, logger: s.testlog}
	sto, apiErr = ctx.getStore(nil, nil)
	c.Check(sto, IsNil)
	c.Check(apiErr, Equals, ErrUnknown)
	c.Check(s.testlog.Captured(), Equals, "ERROR failed to get store: something else\n")
}

var future = time.Now().Add(4 * time.Hour).Format(time.RFC3339)

func (s *handlersSuite) TestCheckBroadcast(c *C) {
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
	store    store.PendingStore
	chanId   store.InternalChannelId
	err      error
	top      int64
	payloads []json.RawMessage
}

func (cbsend *checkBrokerSending) Broadcast(chanId store.InternalChannelId) {
	top, notifications, err := cbsend.store.GetChannelSnapshot(chanId)
	cbsend.err = err
	cbsend.chanId = chanId
	cbsend.top = top
	cbsend.payloads = protocol.ExtractPayloads(notifications)
}

func (s *handlersSuite) TestDoBroadcast(c *C) {
	sto := store.NewInMemoryPendingStore()
	bsend := &checkBrokerSending{store: sto}
	bh := &BroadcastHandler{&context{nil, bsend, nil}}
	payload := json.RawMessage(`{"a": 1}`)
	apiErr := bh.doBroadcast(sto, &Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     payload,
	})
	c.Check(apiErr, IsNil)
	c.Check(bsend.err, IsNil)
	c.Check(bsend.chanId, Equals, store.SystemInternalChannelId)
	c.Check(bsend.top, Equals, int64(1))
	c.Check(bsend.payloads, DeepEquals, []json.RawMessage{payload})
}

func (s *handlersSuite) TestDoBroadcastUnknownChannel(c *C) {
	sto := store.NewInMemoryPendingStore()
	bh := &BroadcastHandler{}
	apiErr := bh.doBroadcast(sto, &Broadcast{
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

func (isto *interceptInMemoryPendingStore) GetInternalChannelId(channel string) (store.InternalChannelId, error) {
	chanId, err := isto.InMemoryPendingStore.GetInternalChannelId(channel)
	return chanId, isto.intercept("GetInternalChannelId", err)
}

func (isto *interceptInMemoryPendingStore) AppendToChannel(chanId store.InternalChannelId, payload json.RawMessage, expiration time.Time) error {
	err := isto.InMemoryPendingStore.AppendToChannel(chanId, payload, expiration)
	return isto.intercept("AppendToChannel", err)
}

func (s *handlersSuite) TestDoBroadcastUnknownError(c *C) {
	sto := &interceptInMemoryPendingStore{
		store.NewInMemoryPendingStore(),
		func(meth string, err error) error {
			return errors.New("other")
		},
	}
	bh := &BroadcastHandler{}
	apiErr := bh.doBroadcast(sto, &Broadcast{
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
	bh := &BroadcastHandler{ctx}
	apiErr := bh.doBroadcast(sto, &Broadcast{
		Channel:  "system",
		ExpireOn: future,
		Data:     json.RawMessage(`{"a": 1}`),
	})
	c.Check(apiErr, Equals, ErrCouldNotStoreNotification)
	c.Check(s.testlog.Captured(), Equals, "ERROR could not store notification: fail\n")
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

func (s *handlersSuite) TestRespondsToBasicSystemBroadcast(c *C) {
	sto := store.NewInMemoryPendingStore()
	stoForReq := func(http.ResponseWriter, *http.Request) (store.PendingStore, error) {
		return sto, nil
	}
	bsend := testBrokerSending{make(chan store.InternalChannelId, 1)}
	testServer := httptest.NewServer(MakeHandlersMux(stoForReq, bsend, nil))
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
	c.Check(dest, DeepEquals, map[string]bool{"ok": true})

	top, _, err := sto.GetChannelSnapshot(store.SystemInternalChannelId)
	c.Assert(err, IsNil)
	c.Check(top, Equals, int64(1))
	c.Check(<-bsend.chanId, Equals, store.SystemInternalChannelId)
}

func (s *handlersSuite) TestStoreUnavailable(c *C) {
	stoForReq := func(http.ResponseWriter, *http.Request) (store.PendingStore, error) {
		return nil, ErrStoreUnavailable
	}
	testServer := httptest.NewServer(MakeHandlersMux(stoForReq, nil, nil))
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
	stoForReq := func(http.ResponseWriter, *http.Request) (store.PendingStore, error) {
		return sto, nil
	}
	testServer := httptest.NewServer(MakeHandlersMux(stoForReq, nil, nil))
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
	stoForReq := func(http.ResponseWriter, *http.Request) (store.PendingStore, error) {
		return store.NewInMemoryPendingStore(), nil
	}
	ctx := &context{stoForReq, nil, nil}
	testServer := httptest.NewServer(&BroadcastHandler{ctx})
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
	stoForReq := func(http.ResponseWriter, *http.Request) (store.PendingStore, error) {
		return store.NewInMemoryPendingStore(), nil
	}
	ctx := &context{stoForReq, nil, nil}
	testServer := httptest.NewServer(&BroadcastHandler{ctx})
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
	testServer := httptest.NewServer(&BroadcastHandler{})
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
	testServer := httptest.NewServer(&BroadcastHandler{})
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
	testServer := httptest.NewServer(&BroadcastHandler{})
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
	testServer := httptest.NewServer(&BroadcastHandler{})
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
	testServer := httptest.NewServer(&BroadcastHandler{})
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
