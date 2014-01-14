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

package main

import (
	// "fmt"
	"bytes"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDevserver(t *testing.T) { TestingT(t) }

type devserverSuite struct{}

var _ = Suite(&devserverSuite{})

func (s *devserverSuite) TestConfigRead(c *C) {
	tmpDir := c.MkDir()
	err := ioutil.WriteFile(filepath.Join(tmpDir, "key.key"), []byte("KeY"), os.ModePerm)
	c.Assert(err, IsNil)
	err = ioutil.WriteFile(filepath.Join(tmpDir, "cert.cert"), []byte("CeRt"), os.ModePerm)
	c.Assert(err, IsNil)
	buf := bytes.NewBufferString(`{
"ping_interval": "5m",
"exchange_timeout": "10s",
"session_queue_size": 10,
"broker_queue_size": 100,
"addr": "127.0.0.1:9999",
"key_pem_file": "key.key",
"cert_pem_file": "cert.cert",
"http_addr": "127.0.0.1:8080",
"http_read_timeout": "5s",
"http_write_timeout": "10s"
}`)
	cfg := &configuration{}
	err = cfg.read(buf, tmpDir)
	c.Assert(err, IsNil)
	c.Check(cfg.PingInterval(), Equals, 5*time.Minute)
	c.Check(cfg.ExchangeTimeout(), Equals, 10*time.Second)
	c.Check(cfg.BrokerQueueSize(), Equals, uint(100))
	c.Check(cfg.SessionQueueSize(), Equals, uint(10))
	c.Check(cfg.Addr(), Equals, "127.0.0.1:9999")
	c.Check(string(cfg.KeyPEMBlock()), Equals, "KeY")
	c.Check(string(cfg.CertPEMBlock()), Equals, "CeRt")
	c.Check(cfg.HTTPAddr(), Equals, "127.0.0.1:8080")
	c.Check(cfg.HTTPReadTimeout(), Equals, 5*time.Second)
	c.Check(cfg.HTTPWriteTimeout(), Equals, 10*time.Second)
}

func (s *devserverSuite) TestConfigReadErrors(c *C) {
	tmpDir := c.MkDir()
	checkError := func(config, expectedErr string) {
		cfg := &configuration{}
		err := cfg.read(bytes.NewBufferString(config), tmpDir)
		c.Check(err, ErrorMatches, expectedErr)
	}
	checkError("", "EOF")
	checkError(`{"ping_interval": "1m"}`, "missing exchange_timeout")
	checkError(`{"ping_interval": "1m", "exchange_timeout": "5s", "session_queue_size": "foo"}`, "session_queue_size:.*type uint")
	checkError(`{
"exchange_timeout": "10s",
"ping_interval": "5m",
"broker_queue_size": 100,
"session_queue_size": 10,
"addr": ":9000",
"key_pem_file": "doesntexist",
"cert_pem_file": "doesntexist",
"http_addr": ":8080",
"http_read_timeout": "5s",
"http_write_timeout": "10s"
}`, "reading key_pem_file:.*no such file.*")
	err := ioutil.WriteFile(filepath.Join(tmpDir, "key.key"), []byte("KeY"), os.ModePerm)
	c.Assert(err, IsNil)
	checkError(`{
"exchange_timeout": "10s",
"ping_interval": "5m",
"broker_queue_size": 100,
"session_queue_size": 10,
"addr": ":9000",
"key_pem_file": "key.key",
"cert_pem_file": "doesntexist",
"http_addr": ":8080",
"http_read_timeout": "5s",
"http_write_timeout": "10s"
}`, "reading cert_pem_file:.*no such file.*")
}
