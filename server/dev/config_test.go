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
	"bytes"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/config"
	"os"
	"path/filepath"
	"time"
)

type configSuite struct{}

var _ = Suite(&configSuite{})

func (s *configSuite) TestDevicesParsedConfig(c *C) {
	buf := bytes.NewBufferString(`{
"ping_interval": "5m",
"exchange_timeout": "10s",
"session_queue_size": 10,
"broker_queue_size": 100,
"addr": "127.0.0.1:9999",
"key_pem_file": "key.key",
"cert_pem_file": "cert.cert"
}`)
	cfg := &DevicesParsedConfig{}
	err := config.ReadConfig(buf, cfg)
	c.Assert(err, IsNil)
	c.Check(cfg.PingInterval(), Equals, 5*time.Minute)
	c.Check(cfg.ExchangeTimeout(), Equals, 10*time.Second)
	c.Check(cfg.BrokerQueueSize(), Equals, uint(100))
	c.Check(cfg.SessionQueueSize(), Equals, uint(10))
	c.Check(cfg.Addr(), Equals, "127.0.0.1:9999")
}

func (s *configSuite) TestDevicesParsedConfigLoadFinish(c *C) {
	tmpDir := c.MkDir()
	cfg := &DevicesParsedConfig{
		ParsedKeyPEMFile:  "key.key",
		ParsedCertPEMFile: "cert.cert",
	}
	err := cfg.FinishLoad(tmpDir)
	c.Check(err, ErrorMatches, "reading key_pem_file:.*no such file.*")
	err = ioutil.WriteFile(filepath.Join(tmpDir, "key.key"), []byte("KeY"), os.ModePerm)
	c.Assert(err, IsNil)
	err = cfg.FinishLoad(tmpDir)
	c.Check(err, ErrorMatches, "reading cert_pem_file:.*no such file.*")
	err = ioutil.WriteFile(filepath.Join(tmpDir, "cert.cert"), []byte("CeRt"), os.ModePerm)
	c.Assert(err, IsNil)
	err = cfg.FinishLoad(tmpDir)
	c.Assert(err, IsNil)
	c.Check(string(cfg.KeyPEMBlock()), Equals, "KeY")
	c.Check(string(cfg.CertPEMBlock()), Equals, "CeRt")
}
