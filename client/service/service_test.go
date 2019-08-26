/*
 Copyright 2014 Canonical Ltd.

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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	. "launchpad.net/gocheck"

	"github.com/ubports/ubuntu-push/bus"
	testibus "github.com/ubports/ubuntu-push/bus/testing"
	"github.com/ubports/ubuntu-push/click"
	"github.com/ubports/ubuntu-push/logger"
	"github.com/ubports/ubuntu-push/nih"
	helpers "github.com/ubports/ubuntu-push/testing"
	"github.com/ubports/ubuntu-push/testing/condition"
)

func TestService(t *testing.T) {
	TestingT(t)
}

type serviceSuite struct {
	log logger.Logger
	bus bus.Endpoint
}

var _ = Suite(&serviceSuite{})

var (
	aPackage      = "com.example.test"
	anAppId       = aPackage + "_test-number-one"
	aPackageOnBus = "/" + string(nih.Quote([]byte(aPackage)))
)

func (ss *serviceSuite) SetUpTest(c *C) {
	ss.log = helpers.NewTestLogger(c, "debug")
	ss.bus = testibus.NewTestingEndpoint(condition.Work(true), nil)
}

var testSetup = &PushServiceSetup{}

func (ss *serviceSuite) TestBuild(c *C) {
	setup := &PushServiceSetup{
		RegURL:   helpers.ParseURL("http://reg"),
		DeviceId: "FOO",
	}
	svc := NewPushService(setup, ss.log)
	c.Check(svc.regURL, DeepEquals, helpers.ParseURL("http://reg"))
	// ...
}

func (ss *serviceSuite) TestStart(c *C) {
	svc := NewPushService(testSetup, ss.log)
	svc.Bus = ss.bus
	c.Check(svc.IsRunning(), Equals, false)
	c.Check(svc.Start(), IsNil)
	c.Check(svc.IsRunning(), Equals, true)
	svc.Stop()
}

func (ss *serviceSuite) TestStartTwice(c *C) {
	svc := NewPushService(testSetup, ss.log)
	svc.Bus = ss.bus
	c.Check(svc.Start(), IsNil)
	c.Check(svc.Start(), Equals, ErrAlreadyStarted)
	svc.Stop()
}

func (ss *serviceSuite) TestStartNoLog(c *C) {
	svc := NewPushService(testSetup, nil)
	svc.Bus = ss.bus
	c.Check(svc.Start(), Equals, ErrNotConfigured)
}

func (ss *serviceSuite) TestStartNoBus(c *C) {
	svc := NewPushService(testSetup, ss.log)
	svc.Bus = nil
	c.Check(svc.Start(), Equals, ErrNotConfigured)
}

func (ss *serviceSuite) TestStartFailsOnBusDialFailure(c *C) {
	bus := testibus.NewTestingEndpoint(condition.Work(false), nil)
	svc := NewPushService(testSetup, ss.log)
	svc.Bus = bus
	c.Check(svc.Start(), ErrorMatches, `.*(?i)cond said no.*`)
	svc.Stop()
}

func (ss *serviceSuite) TestStartGrabsName(c *C) {
	svc := NewPushService(testSetup, ss.log)
	svc.Bus = ss.bus
	c.Assert(svc.Start(), IsNil)
	callArgs := testibus.GetCallArgs(ss.bus)
	defer svc.Stop()
	c.Assert(callArgs, NotNil)
	c.Check(callArgs[0].Member, Equals, "::GrabName")
}

func (ss *serviceSuite) TestStopClosesBus(c *C) {
	svc := NewPushService(testSetup, ss.log)
	svc.Bus = ss.bus
	c.Assert(svc.Start(), IsNil)
	svc.Stop()
	callArgs := testibus.GetCallArgs(ss.bus)
	c.Assert(callArgs, NotNil)
	c.Check(callArgs[len(callArgs)-1].Member, Equals, "::Close")
}

// registration tests

func (ss *serviceSuite) TestGetRegUrlWorks(c *C) {
	setup := &PushServiceSetup{
		RegURL: helpers.ParseURL("http://foo"),
	}
	svc := NewPushService(setup, ss.log)
	svc.Bus = ss.bus
	url := svc.getParsedUrl("/op")
	c.Check(url, Equals, "http://foo/op")
}

func (ss *serviceSuite) TestGetRegUrlDoesNotPanic(c *C) {
	svc := NewPushService(testSetup, ss.log)
	svc.Bus = ss.bus
	url := svc.getParsedUrl("/op")
	c.Check(url, Equals, "")
}

func (ss *serviceSuite) TestRegistrationAndUnregistrationFailIfBadArgs(c *C) {
	for i, s := range []struct {
		args []interface{}
		errt error
	}{
		{nil, ErrBadArgCount},
		{[]interface{}{}, ErrBadArgCount},
		{[]interface{}{1}, ErrBadArgType},
		{[]interface{}{"foo"}, click.ErrInvalidAppId},
		{[]interface{}{"foo", "bar"}, ErrBadArgCount},
	} {
		reg, err := new(PushService).register("/bar", s.args, nil)
		c.Check(reg, IsNil, Commentf("iteration #%d", i))
		c.Check(err, Equals, s.errt, Commentf("iteration #%d", i))

		reg, err = new(PushService).unregister("/bar", s.args, nil)
		c.Check(reg, IsNil, Commentf("iteration #%d", i))
		c.Check(err, Equals, s.errt, Commentf("iteration #%d", i))
	}
}

func (ss *serviceSuite) TestRegistrationWorks(c *C) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 256)
		n := r.ContentLength
		_, e := io.ReadFull(r.Body, buf[:n])
		c.Assert(e, IsNil)
		req := registrationRequest{}
		c.Assert(json.Unmarshal(buf[:n], &req), IsNil)
		c.Check(req, DeepEquals, registrationRequest{"fake-device-id", anAppId})
		c.Check(r.URL.Path, Equals, "/register")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"ok":true,"token":"blob-of-bytes"}`)
	}))
	defer ts.Close()
	setup := &PushServiceSetup{
		DeviceId:   "fake-device-id",
		RegURL:     helpers.ParseURL(ts.URL),
	}
	svc := NewPushService(setup, ss.log)
	svc.Bus = ss.bus
	// this'll check (un)quoting, too
	reg, err := svc.register(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Assert(err, IsNil)
	c.Assert(reg, HasLen, 1)
	regs, ok := reg[0].(string)
	c.Check(ok, Equals, true)
	c.Check(regs, Equals, "blob-of-bytes")
}

func (ss *serviceSuite) TestRegistrationOverrideWorks(c *C) {
	envar := "PUSH_REG_" + string(nih.Quote([]byte(anAppId)))
	os.Setenv(envar, "42")
	defer os.Setenv(envar, "")

	reg, err := new(PushService).register(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Assert(reg, HasLen, 1)
	regs, ok := reg[0].(string)
	c.Check(ok, Equals, true)
	c.Check(regs, Equals, "42")
	c.Check(err, IsNil)
}

func (ss *serviceSuite) TestManageRegFailsOnNoServer(c *C) {
	setup := &PushServiceSetup{
		DeviceId:   "fake-device-id",
		RegURL:     helpers.ParseURL("xyzzy://"),
	}
	svc := NewPushService(setup, ss.log)
	svc.Bus = ss.bus
	reg, err := svc.register(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Check(reg, IsNil)
	c.Check(err, ErrorMatches, "unable to request registration: .*")
}

func (ss *serviceSuite) TestManageRegFailsOn401(c *C) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Unauthorized", 401)
	}))
	defer ts.Close()
	setup := &PushServiceSetup{
		DeviceId:   "fake-device-id",
		RegURL:     helpers.ParseURL(ts.URL),
	}
	svc := NewPushService(setup, ss.log)
	svc.Bus = ss.bus
	reg, err := svc.register(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Check(err, Equals, ErrBadAuth)
	c.Check(reg, IsNil)
}

func (ss *serviceSuite) TestManageRegFailsOn40x(c *C) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "I'm a teapot", 418)
	}))
	defer ts.Close()
	setup := &PushServiceSetup{
		DeviceId:   "fake-device-id",
		RegURL:     helpers.ParseURL(ts.URL),
	}
	svc := NewPushService(setup, ss.log)
	svc.Bus = ss.bus
	reg, err := svc.register(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Check(err, Equals, ErrBadRequest)
	c.Check(reg, IsNil)
}

func (ss *serviceSuite) TestManageRegFailsOn50x(c *C) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not implemented", 501)
	}))
	defer ts.Close()
	setup := &PushServiceSetup{
		DeviceId:   "fake-device-id",
		RegURL:     helpers.ParseURL(ts.URL),
	}
	svc := NewPushService(setup, ss.log)
	svc.Bus = ss.bus
	reg, err := svc.register(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Check(err, Equals, ErrBadServer)
	c.Check(reg, IsNil)
}

func (ss *serviceSuite) TestManageRegFailsOnBadJSON(c *C) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 256)
		n := r.ContentLength
		_, e := io.ReadFull(r.Body, buf[:n])
		c.Assert(e, IsNil)
		req := registrationRequest{}
		c.Assert(json.Unmarshal(buf[:n], &req), IsNil)
		c.Check(req, DeepEquals, registrationRequest{"fake-device-id", anAppId})

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{`)
	}))
	defer ts.Close()
	setup := &PushServiceSetup{
		DeviceId:   "fake-device-id",
		RegURL:     helpers.ParseURL(ts.URL),
	}
	svc := NewPushService(setup, ss.log)
	svc.Bus = ss.bus
	// this'll check (un)quoting, too
	reg, err := svc.register(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Check(reg, IsNil)
	c.Check(err, ErrorMatches, "unable to unmarshal register response: .*")
}

func (ss *serviceSuite) TestManageRegFailsOnBadJSONDocument(c *C) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 256)
		n := r.ContentLength
		_, e := io.ReadFull(r.Body, buf[:n])
		c.Assert(e, IsNil)
		req := registrationRequest{}
		c.Assert(json.Unmarshal(buf[:n], &req), IsNil)
		c.Check(req, DeepEquals, registrationRequest{"fake-device-id", anAppId})

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"bananas": "very yes"}`)
	}))
	defer ts.Close()
	setup := &PushServiceSetup{
		DeviceId:   "fake-device-id",
		RegURL:     helpers.ParseURL(ts.URL),
	}
	svc := NewPushService(setup, ss.log)
	svc.Bus = ss.bus
	// this'll check (un)quoting, too
	reg, err := svc.register(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Check(reg, IsNil)
	c.Check(err, Equals, ErrBadToken)
}

func (ss *serviceSuite) TestDBusUnregisterWorks(c *C) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 256)
		n := r.ContentLength
		_, e := io.ReadFull(r.Body, buf[:n])
		c.Assert(e, IsNil)
		req := registrationRequest{}
		c.Assert(json.Unmarshal(buf[:n], &req), IsNil)
		c.Check(req, DeepEquals, registrationRequest{"fake-device-id", anAppId})
		c.Check(r.URL.Path, Equals, "/unregister")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"ok":true,"token":"blob-of-bytes"}`)
	}))
	defer ts.Close()
	setup := &PushServiceSetup{
		DeviceId:   "fake-device-id",
		RegURL:     helpers.ParseURL(ts.URL),
	}
	svc := NewPushService(setup, ss.log)
	svc.Bus = ss.bus
	// this'll check (un)quoting, too
	reg, err := svc.unregister(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Assert(err, IsNil)
	c.Assert(reg, HasLen, 0)
}

func (ss *serviceSuite) TestUnregistrationWorks(c *C) {
	invoked := make(chan bool, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 256)
		n := r.ContentLength
		_, e := io.ReadFull(r.Body, buf[:n])
		c.Assert(e, IsNil)
		req := registrationRequest{}
		c.Assert(json.Unmarshal(buf[:n], &req), IsNil)
		c.Check(req, DeepEquals, registrationRequest{"fake-device-id", anAppId})
		c.Check(r.URL.Path, Equals, "/unregister")
		invoked <- true
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"ok":true}`)
	}))
	defer ts.Close()
	setup := &PushServiceSetup{
		DeviceId:   "fake-device-id",
		RegURL:     helpers.ParseURL(ts.URL),
	}
	svc := NewPushService(setup, ss.log)
	svc.Bus = ss.bus
	err := svc.Unregister(anAppId)
	c.Assert(err, IsNil)
	c.Check(invoked, HasLen, 1)
}
