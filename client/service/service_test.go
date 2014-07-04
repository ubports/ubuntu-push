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
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/bus"
	testibus "launchpad.net/ubuntu-push/bus/testing"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/nih"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
)

func TestService(t *testing.T) { TestingT(t) }

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
		AuthGetter: func(s string) string {
			return ""
		},
	}
	svc := NewPushService(ss.bus, setup, ss.log)
	c.Check(svc.Bus, Equals, ss.bus)
	c.Check(svc.regURL, DeepEquals, helpers.ParseURL("http://reg"))
	c.Check(fmt.Sprintf("%#v", svc.authGetter), Equals, fmt.Sprintf("%#v", setup.AuthGetter))
	// ...
}

func (ss *serviceSuite) TestStart(c *C) {
	svc := NewPushService(ss.bus, testSetup, ss.log)
	c.Check(svc.IsRunning(), Equals, false)
	c.Check(svc.Start(), IsNil)
	c.Check(svc.IsRunning(), Equals, true)
	svc.Stop()
}

func (ss *serviceSuite) TestStartTwice(c *C) {
	svc := NewPushService(ss.bus, testSetup, ss.log)
	c.Check(svc.Start(), IsNil)
	c.Check(svc.Start(), Equals, ErrAlreadyStarted)
	svc.Stop()
}

func (ss *serviceSuite) TestStartNoLog(c *C) {
	svc := NewPushService(ss.bus, testSetup, nil)
	c.Check(svc.Start(), Equals, ErrNotConfigured)
}

func (ss *serviceSuite) TestStartNoBus(c *C) {
	svc := NewPushService(nil, testSetup, ss.log)
	c.Check(svc.Start(), Equals, ErrNotConfigured)
}

func (ss *serviceSuite) TestStartFailsOnBusDialFailure(c *C) {
	bus := testibus.NewTestingEndpoint(condition.Work(false), nil)
	svc := NewPushService(bus, testSetup, ss.log)
	c.Check(svc.Start(), ErrorMatches, `.*(?i)cond said no.*`)
	svc.Stop()
}

func (ss *serviceSuite) TestStartGrabsName(c *C) {
	svc := NewPushService(ss.bus, testSetup, ss.log)
	c.Assert(svc.Start(), IsNil)
	callArgs := testibus.GetCallArgs(ss.bus)
	defer svc.Stop()
	c.Assert(callArgs, NotNil)
	c.Check(callArgs[0].Member, Equals, "::GrabName")
}

func (ss *serviceSuite) TestStopClosesBus(c *C) {
	svc := NewPushService(ss.bus, testSetup, ss.log)
	c.Assert(svc.Start(), IsNil)
	svc.Stop()
	callArgs := testibus.GetCallArgs(ss.bus)
	c.Assert(callArgs, NotNil)
	c.Check(callArgs[len(callArgs)-1].Member, Equals, "::Close")
}

// registration tests

func (ss *serviceSuite) TestGetRegAuthWorks(c *C) {
	ch := make(chan string, 1)
	setup := &PushServiceSetup{
		RegURL: helpers.ParseURL("http://foo"),
		AuthGetter: func(s string) string {
			ch <- s
			return "Auth " + s
		},
	}
	svc := NewPushService(ss.bus, setup, ss.log)
	url, auth := svc.getAuthorization("/op")
	c.Check(auth, Equals, "Auth http://foo/op")
	c.Assert(len(ch), Equals, 1)
	c.Check(<-ch, Equals, "http://foo/op")
	c.Check(url, Equals, "http://foo/op")
}

func (ss *serviceSuite) TestGetRegAuthDoesNotPanic(c *C) {
	svc := NewPushService(ss.bus, testSetup, ss.log)
	_, auth := svc.getAuthorization("/op")
	c.Check(auth, Equals, "")
}

func (ss *serviceSuite) TestRegistrationAndUnregistrationFailIfBadArgs(c *C) {
	for i, s := range []struct {
		args []interface{}
		errt error
	}{
		{nil, ErrBadArgCount},
		{[]interface{}{}, ErrBadArgCount},
		{[]interface{}{1}, ErrBadArgType},
		{[]interface{}{"foo"}, ErrBadAppId},
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
		n, e := r.Body.Read(buf)
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
		AuthGetter: func(string) string { return "tok" },
	}
	svc := NewPushService(ss.bus, setup, ss.log)
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

func (ss *serviceSuite) TestManageRegFailsOnBadAuth(c *C) {
	// ... no auth added
	svc := NewPushService(ss.bus, testSetup, ss.log)
	reg, err := svc.register(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Check(reg, IsNil)
	c.Check(err, Equals, BadAuth)
}

func (ss *serviceSuite) TestManageRegFailsOnNoServer(c *C) {
	setup := &PushServiceSetup{
		DeviceId:   "fake-device-id",
		RegURL:     helpers.ParseURL("xyzzy://"),
		AuthGetter: func(string) string { return "tok" },
	}
	svc := NewPushService(ss.bus, setup, ss.log)
	reg, err := svc.register(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Check(reg, IsNil)
	c.Check(err, ErrorMatches, "unable to request registration: .*")
}

func (ss *serviceSuite) TestManageRegFailsOn40x(c *C) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "I'm a teapot", 418)
	}))
	defer ts.Close()
	setup := &PushServiceSetup{
		DeviceId:   "fake-device-id",
		RegURL:     helpers.ParseURL(ts.URL),
		AuthGetter: func(string) string { return "tok" },
	}
	svc := NewPushService(ss.bus, setup, ss.log)
	reg, err := svc.register(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Check(err, Equals, BadRequest)
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
		AuthGetter: func(string) string { return "tok" },
	}
	svc := NewPushService(ss.bus, setup, ss.log)
	reg, err := svc.register(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Check(err, Equals, BadServer)
	c.Check(reg, IsNil)
}

func (ss *serviceSuite) TestManageRegFailsOnBadJSON(c *C) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 256)
		n, e := r.Body.Read(buf)
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
		AuthGetter: func(string) string { return "tok" },
	}
	svc := NewPushService(ss.bus, setup, ss.log)
	// this'll check (un)quoting, too
	reg, err := svc.register(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Check(reg, IsNil)
	c.Check(err, ErrorMatches, "unable to unmarshal register response: .*")
}

func (ss *serviceSuite) TestManageRegFailsOnBadJSONDocument(c *C) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 256)
		n, e := r.Body.Read(buf)
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
		AuthGetter: func(string) string { return "tok" },
	}
	svc := NewPushService(ss.bus, setup, ss.log)
	// this'll check (un)quoting, too
	reg, err := svc.register(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Check(reg, IsNil)
	c.Check(err, Equals, BadToken)
}

func (ss *serviceSuite) TestDBusUnregisterWorks(c *C) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 256)
		n, e := r.Body.Read(buf)
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
		AuthGetter: func(string) string { return "tok" },
	}
	svc := NewPushService(ss.bus, setup, ss.log)
	// this'll check (un)quoting, too
	reg, err := svc.unregister(aPackageOnBus, []interface{}{anAppId}, nil)
	c.Assert(err, IsNil)
	c.Assert(reg, HasLen, 0)
}

func (ss *serviceSuite) TestUnregistrationWorks(c *C) {
	invoked := make(chan bool, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 256)
		n, e := r.Body.Read(buf)
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
		AuthGetter: func(string) string { return "tok" },
	}
	svc := NewPushService(ss.bus, setup, ss.log)
	err := svc.Unregister(anAppId)
	c.Assert(err, IsNil)
	c.Check(invoked, HasLen, 1)
}
