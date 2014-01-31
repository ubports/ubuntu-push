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

package session

import (
	"errors"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/logger"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/testing/condition"
	"net"
	"os"
	"testing"
	"time"
)

func TestSession(t *testing.T) { TestingT(t) }

type clientSessionSuite struct{}

var nullog = logger.NewSimpleLogger(ioutil.Discard, "error")
var debuglog = logger.NewSimpleLogger(os.Stderr, "debug")
var _ = Suite(&clientSessionSuite{})

//
// helpers! candidates to live in their own ../testing/ package.
//

type xAddr string

func (x xAddr) Network() string { return "<:>" }
func (x xAddr) String() string  { return string(x) }

// testConn (roughly based on the one in protocol_test)

type testConn struct {
	Name              string
	Deadlines         []time.Duration
	Writes            [][]byte
	WriteCondition    condition.Interface
	DeadlineCondition condition.Interface
	CloseCondition    condition.Interface
}

func (tc *testConn) LocalAddr() net.Addr { return xAddr(tc.Name) }

func (tc *testConn) RemoteAddr() net.Addr { return xAddr(tc.Name) }

func (tc *testConn) Close() error {
	if tc.CloseCondition == nil || tc.CloseCondition.OK() {
		return nil
	} else {
		return errors.New("closer on fire")
	}
}

func (tc *testConn) SetDeadline(t time.Time) error      { panic("SetDeadline not implemented.") }
func (tc *testConn) SetReadDeadline(t time.Time) error  { panic("SetReadDeadline not implemented.") }
func (tc *testConn) SetWriteDeadline(t time.Time) error { panic("SetWriteDeadline not implemented.") }
func (tc *testConn) Read(buf []byte) (n int, err error) { panic("Read not implemented.") }
func (tc *testConn) Write(buf []byte) (int, error)      { panic("Write not implemented.") }

/****************************************************************
  NewSession() tests
****************************************************************/

func (cs *clientSessionSuite) TestNewSessionPlainWorks(c *C) {
	sess, err := NewSession("", nil, 0, "", nullog)
	c.Check(sess, NotNil)
	c.Check(err, IsNil)
	// but no root CAs set
	c.Check(sess.TLS.RootCAs, IsNil)
}

var certfile string = helpers.SourceRelative("../../server/acceptance/config/testing.cert")
var pem, _ = ioutil.ReadFile(certfile)

func (cs *clientSessionSuite) TestNewSessionPEMWorks(c *C) {
	sess, err := NewSession("", pem, 0, "wah", nullog)
	c.Check(sess, NotNil)
	c.Assert(err, IsNil)
	c.Check(sess.TLS.RootCAs, NotNil)
}

func (cs *clientSessionSuite) TestNewSessionBadPEMFileContentFails(c *C) {
	badpem := []byte("This is not the PEM you're looking for.")
	sess, err := NewSession("", badpem, 0, "wah", nullog)
	c.Check(sess, IsNil)
	c.Check(err, NotNil)
}

/****************************************************************
  Dial() tests
****************************************************************/

func (cs *clientSessionSuite) TestDialFailsWithNoAddress(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	err = sess.Dial()
	c.Assert(err, NotNil)
	c.Check(err.Error(), Matches, ".*dial.*address.*")
}

func (cs *clientSessionSuite) TestDialConnects(c *C) {
	srv, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, IsNil)
	defer srv.Close()
	sess, err := NewSession(srv.Addr().String(), nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	err = sess.Dial()
	c.Check(err, IsNil)
	c.Check(sess.Connection, NotNil)
}

func (cs *clientSessionSuite) TestDialConnectFail(c *C) {
	srv, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, IsNil)
	sess, err := NewSession(srv.Addr().String(), nil, 0, "wah", debuglog)
	srv.Close()
	c.Assert(err, IsNil)
	err = sess.Dial()
	c.Check(sess.Connection, IsNil)
	c.Assert(err, NotNil)
	c.Check(err.Error(), Matches, ".*connection refused")
}

/****************************************************************
  Close() tests
****************************************************************/

func (cs *clientSessionSuite) TestClose(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: "TestClose"}
	sess.Close()
	c.Check(sess.Connection, IsNil)
}

func (cs *clientSessionSuite) TestCloseTwice(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: "TestCloseTwice"}
	sess.Close()
	c.Check(sess.Connection, IsNil)
	sess.Close()
	c.Check(sess.Connection, IsNil)
}

func (cs *clientSessionSuite) TestCloseFails(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	sess.Connection = &testConn{Name: "TestCloseFails", CloseCondition: condition.Work(false)}
	sess.Close()
	c.Check(sess.Connection, IsNil) // nothing you can do to clean up anyway
}

/****************************************************************
  checkRunnable() tests
****************************************************************/

func (cs *clientSessionSuite) TestCheckRunnableFailsIfNoConnection(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	// no connection!
	c.Check(sess.checkRunnable(), NotNil)
}

func (cs *clientSessionSuite) TestCheckRunnableFailsIfNoProtocolator(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	// set up the connection
	sess.Connection = &testConn{}
	// And stomp on the protocolator
	sess.Protocolator = nil
	c.Check(sess.checkRunnable(), NotNil)
}

func (cs *clientSessionSuite) TestCheckRunnable(c *C) {
	sess, err := NewSession("", nil, 0, "wah", debuglog)
	c.Assert(err, IsNil)
	// set up the connection
	sess.Connection = &testConn{}
	c.Check(sess.checkRunnable(), IsNil)
}
