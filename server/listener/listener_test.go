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

package listener

import (
	"crypto/tls"
	"io"
	"net"
	"os/exec"
	"regexp"
	"syscall"
	"testing"
	"time"
	"unicode"

	. "launchpad.net/gocheck"

	helpers "launchpad.net/ubuntu-push/testing"
)

func TestListener(t *testing.T) { TestingT(t) }

type listenerSuite struct {
	testlog *helpers.TestLogger
}

var _ = Suite(&listenerSuite{})

const NofileMax = 20

func (s *listenerSuite) SetUpSuite(*C) {
	// make it easier to get a too many open files error
	var nofileLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &nofileLimit)
	if err != nil {
		panic(err)
	}
	nofileLimit.Cur = NofileMax
	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &nofileLimit)
	if err != nil {
		panic(err)
	}
}

func (s *listenerSuite) SetUpTest(c *C) {
	s.testlog = helpers.NewTestLogger(c, "error")
}

type testDevListenerCfg struct {
	addr string
}

func (cfg *testDevListenerCfg) Addr() string {
	return cfg.addr
}

func (cfg *testDevListenerCfg) TLSServerConfig() *tls.Config {
	return helpers.TestTLSServerConfig
}

func (s *listenerSuite) TestDeviceListen(c *C) {
	lst, err := DeviceListen(nil, &testDevListenerCfg{"127.0.0.1:0"})
	c.Check(err, IsNil)
	defer lst.Close()
	c.Check(lst.Addr().String(), Matches, `127.0.0.1:\d{5}`)
}

func (s *listenerSuite) TestDeviceListenError(c *C) {
	// assume tests are not running as root
	_, err := DeviceListen(nil, &testDevListenerCfg{"127.0.0.1:99"})
	c.Check(err, ErrorMatches, ".*permission denied.*")
}

type testNetError struct {
	temp bool
}

func (tne *testNetError) Error() string {
	return "test net error"
}

func (tne *testNetError) Temporary() bool {
	return tne.temp
}

func (tne *testNetError) Timeout() bool {
	return false
}

var _ net.Error = &testNetError{} // sanity check

func (s *listenerSuite) TestHandleTemporary(c *C) {
	c.Check(handleTemporary(&testNetError{true}), Equals, true)
	c.Check(handleTemporary(&testNetError{false}), Equals, false)
}

func testSession(conn net.Conn) error {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	var buf [1]byte
	for {
		_, err := io.ReadFull(conn, buf[:])
		if err != nil {
			return err
		}
		// 1|2... send digit back
		if unicode.IsDigit(rune(buf[0])) {
			break
		}
	}
	_, err := conn.Write(buf[:])
	return err
}

func testTlsDial(addr string) (net.Conn, error) {
	return tls.Dial("tcp", addr, helpers.TestTLSClientConfig)
}

func testWriteByte(c *C, conn net.Conn, toWrite uint32) {
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, err := conn.Write([]byte{byte(toWrite)})
	c.Assert(err, IsNil)
}

func testReadByte(c *C, conn net.Conn, expected uint32) {
	var buf [1]byte
	_, err := io.ReadFull(conn, buf[:])
	c.Check(err, IsNil)
	c.Check(buf[0], Equals, byte(expected))
}

type testSessionResourceManager struct {
	event chan string
}

func (r *testSessionResourceManager) ConsumeConn() {
	r.event <- "consume"
}

// takeNext takes a string from given channel with a 5s timeout
func takeNext(ch <-chan string) string {
	select {
	case <-time.After(5 * time.Second):
		panic("test protocol exchange stuck: too long waiting")
	case v := <-ch:
		return v
	}
}

func (s *listenerSuite) TestDeviceAcceptLoop(c *C) {
	lst, err := DeviceListen(nil, &testDevListenerCfg{"127.0.0.1:0"})
	c.Check(err, IsNil)
	defer lst.Close()
	errCh := make(chan error)
	rEvent := make(chan string)
	resource := &testSessionResourceManager{rEvent}
	go func() {
		errCh <- lst.AcceptLoop(testSession, resource, s.testlog)
	}()
	listenerAddr := lst.Addr().String()
	c.Check(takeNext(rEvent), Equals, "consume")
	conn1, err := testTlsDial(listenerAddr)
	c.Assert(err, IsNil)
	c.Check(takeNext(rEvent), Equals, "consume")
	defer conn1.Close()
	testWriteByte(c, conn1, '1')
	conn2, err := testTlsDial(listenerAddr)
	c.Assert(err, IsNil)
	c.Check(takeNext(rEvent), Equals, "consume")
	defer conn2.Close()
	testWriteByte(c, conn2, '2')
	testReadByte(c, conn1, '1')
	testReadByte(c, conn2, '2')
	lst.Close()
	c.Check(<-errCh, ErrorMatches, ".*use of closed.*")
	c.Check(s.testlog.Captured(), Equals, "")
}

// waitForLogs waits for the logs captured in s.testlog to match reStr.
func (s *listenerSuite) waitForLogs(c *C, reStr string) {
	rx := regexp.MustCompile("^" + reStr + "$")
	for i := 0; i < 100; i++ {
		if rx.MatchString(s.testlog.Captured()) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	c.Check(s.testlog.Captured(), Matches, reStr)
}

func (s *listenerSuite) TestDeviceAcceptLoopTemporaryError(c *C) {
	// ENFILE is not the temp network error we want to handle this way
	// but is relatively easy to generate in a controlled way
	var err error
	lst, err := DeviceListen(nil, &testDevListenerCfg{"127.0.0.1:0"})
	c.Check(err, IsNil)
	defer lst.Close()
	errCh := make(chan error)
	resource := &NopSessionResourceManager{}
	go func() {
		errCh <- lst.AcceptLoop(testSession, resource, s.testlog)
	}()
	listenerAddr := lst.Addr().String()
	connectMany := helpers.ScriptAbsPath("connect-many.py")
	cmd := exec.Command(connectMany, listenerAddr)
	res, err := cmd.Output()
	c.Assert(err, IsNil)
	c.Assert(string(res), Matches, "(?s).*timed out.*")
	conn2, err := testTlsDial(listenerAddr)
	c.Assert(err, IsNil)
	defer conn2.Close()
	testWriteByte(c, conn2, '2')
	testReadByte(c, conn2, '2')
	lst.Close()
	c.Check(<-errCh, ErrorMatches, ".*use of closed.*")
	s.waitForLogs(c, "(?ms).*device listener:.*accept.*too many open.*-- retrying")
}

func (s *listenerSuite) TestDeviceAcceptLoopPanic(c *C) {
	lst, err := DeviceListen(nil, &testDevListenerCfg{"127.0.0.1:0"})
	c.Check(err, IsNil)
	defer lst.Close()
	errCh := make(chan error)
	resource := &NopSessionResourceManager{}
	go func() {
		errCh <- lst.AcceptLoop(func(conn net.Conn) error {
			defer conn.Close()
			panic("session crash")
		}, resource, s.testlog)
	}()
	listenerAddr := lst.Addr().String()
	_, err = testTlsDial(listenerAddr)
	c.Assert(err, Not(IsNil))
	lst.Close()
	c.Check(<-errCh, ErrorMatches, ".*use of closed.*")
	s.waitForLogs(c, "(?s)ERROR\\(PANIC\\) terminating device connection on: session crash:.*AcceptLoop.*")
}

func (s *listenerSuite) TestForeignListener(c *C) {
	foreignLst, err := net.Listen("tcp", "127.0.0.1:0")
	c.Check(err, IsNil)
	lst, err := DeviceListen(foreignLst, &testDevListenerCfg{"127.0.0.1:0"})
	c.Check(err, IsNil)
	defer lst.Close()
	errCh := make(chan error)
	resource := &NopSessionResourceManager{}
	go func() {
		errCh <- lst.AcceptLoop(testSession, resource, s.testlog)
	}()
	listenerAddr := lst.Addr().String()
	c.Check(listenerAddr, Equals, foreignLst.Addr().String())
	conn1, err := testTlsDial(listenerAddr)
	c.Assert(err, IsNil)
	defer conn1.Close()
	testWriteByte(c, conn1, '1')
	testReadByte(c, conn1, '1')
	lst.Close()
	c.Check(<-errCh, ErrorMatches, ".*use of closed.*")
	c.Check(s.testlog.Captured(), Equals, "")
}
