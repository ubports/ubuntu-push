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
	"crypto/x509"
	"net"
	"syscall"
	"testing"
	"time"

	. "launchpad.net/gocheck"

	helpers "launchpad.net/ubuntu-push/testing"
)

func TestListener(t *testing.T) { TestingT(t) }

type listenerSuite struct {
	testlog *helpers.TestLogger
}

var _ = Suite(&listenerSuite{})

const NofileMax = 500

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

func (cfg *testDevListenerCfg) KeyPEMBlock() []byte {
	return helpers.TestKeyPEMBlock
}

func (cfg *testDevListenerCfg) CertPEMBlock() []byte {
	return helpers.TestCertPEMBlock
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
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	var buf [1]byte
	_, err := conn.Read(buf[:])
	if err != nil {
		return err
	}
	_, err = conn.Write(buf[:])
	return err
}

func testTlsDial(c *C, addr string) (net.Conn, error) {
	cp := x509.NewCertPool()
	ok := cp.AppendCertsFromPEM((&testDevListenerCfg{}).CertPEMBlock())
	c.Assert(ok, Equals, true)
	return tls.Dial("tcp", addr, &tls.Config{RootCAs: cp})
}

func testWriteByte(c *C, conn net.Conn, toWrite uint32) {
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, err := conn.Write([]byte{byte(toWrite)})
	c.Assert(err, IsNil)
}

func testReadByte(c *C, conn net.Conn, expected uint32) {
	var buf [1]byte
	_, err := conn.Read(buf[:])
	c.Check(err, IsNil)
	c.Check(buf[0], Equals, byte(expected))
}

func (s *listenerSuite) TestDeviceAcceptLoop(c *C) {
	lst, err := DeviceListen(nil, &testDevListenerCfg{"127.0.0.1:0"})
	c.Check(err, IsNil)
	defer lst.Close()
	errCh := make(chan error)
	go func() {
		errCh <- lst.AcceptLoop(testSession, s.testlog)
	}()
	listenerAddr := lst.Addr().String()
	conn1, err := testTlsDial(c, listenerAddr)
	c.Assert(err, IsNil)
	defer conn1.Close()
	testWriteByte(c, conn1, '1')
	conn2, err := testTlsDial(c, listenerAddr)
	c.Assert(err, IsNil)
	defer conn2.Close()
	testWriteByte(c, conn2, '2')
	testReadByte(c, conn1, '1')
	testReadByte(c, conn2, '2')
	lst.Close()
	c.Check(<-errCh, ErrorMatches, ".*use of closed.*")
	c.Check(s.testlog.Captured(), Equals, "")
}

func (s *listenerSuite) TestDeviceAcceptLoopTemporaryError(c *C) {
	// ENFILE is not the temp network error we want to handle this way
	// but is relatively easy to generate in a controlled way
	var err error
	lst, err := DeviceListen(nil, &testDevListenerCfg{"127.0.0.1:0"})
	c.Check(err, IsNil)
	defer lst.Close()
	errCh := make(chan error)
	go func() {
		errCh <- lst.AcceptLoop(testSession, s.testlog)
	}()
	listenerAddr := lst.Addr().String()
	conns := make([]net.Conn, 0, NofileMax)
	for i := 0; i < NofileMax; i++ {
		var conn1 net.Conn
		conn1, err = net.Dial("tcp", listenerAddr)
		if err != nil {
			break
		}
		defer conn1.Close()
		conns = append(conns, conn1)
	}
	c.Assert(err, ErrorMatches, "*.too many open.*")
	for _, conn := range conns {
		conn.Close()
	}
	conn2, err := testTlsDial(c, listenerAddr)
	c.Assert(err, IsNil)
	defer conn2.Close()
	testWriteByte(c, conn2, '2')
	testReadByte(c, conn2, '2')
	lst.Close()
	c.Check(<-errCh, ErrorMatches, ".*use of closed.*")
	c.Check(s.testlog.Captured(), Matches, ".*device listener:.*accept.*too many open.*-- retrying\n")
}

func (s *listenerSuite) TestDeviceAcceptLoopPanic(c *C) {
	lst, err := DeviceListen(nil, &testDevListenerCfg{"127.0.0.1:0"})
	c.Check(err, IsNil)
	defer lst.Close()
	errCh := make(chan error)
	go func() {
		errCh <- lst.AcceptLoop(func(conn net.Conn) error {
			defer conn.Close()
			panic("session crash")
		}, s.testlog)
	}()
	listenerAddr := lst.Addr().String()
	_, err = testTlsDial(c, listenerAddr)
	c.Assert(err, Not(IsNil))
	lst.Close()
	c.Check(<-errCh, ErrorMatches, ".*use of closed.*")
	c.Check(s.testlog.Captured(), Matches, "(?s)ERROR\\(PANIC\\) terminating device connection on: session crash:.*AcceptLoop.*")
}

func (s *listenerSuite) TestForeignListener(c *C) {
	foreignLst, err := net.Listen("tcp", "127.0.0.1:0")
	c.Check(err, IsNil)
	lst, err := DeviceListen(foreignLst, &testDevListenerCfg{"127.0.0.1:0"})
	c.Check(err, IsNil)
	defer lst.Close()
	errCh := make(chan error)
	go func() {
		errCh <- lst.AcceptLoop(testSession, s.testlog)
	}()
	listenerAddr := lst.Addr().String()
	c.Check(listenerAddr, Equals, foreignLst.Addr().String())
	conn1, err := testTlsDial(c, listenerAddr)
	c.Assert(err, IsNil)
	defer conn1.Close()
	testWriteByte(c, conn1, '1')
	testReadByte(c, conn1, '1')
	lst.Close()
	c.Check(<-errCh, ErrorMatches, ".*use of closed.*")
	c.Check(s.testlog.Captured(), Equals, "")
}
