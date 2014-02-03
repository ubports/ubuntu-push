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

package client

import (
	"fmt"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/logger"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/whoopsie/identifier"
	idtesting "launchpad.net/ubuntu-push/whoopsie/identifier/testing"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClient(t *testing.T) { TestingT(t) }

type clientSuite struct {
	configPath string
}

var nullog = logger.NewSimpleLogger(ioutil.Discard, "error")
var noisylog = logger.NewSimpleLogger(os.Stderr, "debug")
var debuglog = nullog
var _ = Suite(&clientSuite{})

func (cs *clientSuite) SetUpTest(c *C) {
	dir := c.MkDir()
	cs.configPath = filepath.Join(dir, "config")
	cfg := fmt.Sprintf(`
{
    "exchange_timeout": "10ms",
    "stabilizing_timeout": "0ms",
    "connectivity_check_url": "",
    "connectivity_check_md5": "",
    "addr": ":0",
    "cert_pem_file": %#v,
    "recheck_timeout": "3h"
}`, helpers.SourceRelative("../server/acceptance/config/testing.cert"))
	ioutil.WriteFile(cs.configPath, []byte(cfg), 0600)
}

/*****************************************************************
    Configure tests
******************************************************************/

func (cs *clientSuite) TestConfigureWorks(c *C) {
	cli := new(Client)
	err := cli.Configure(cs.configPath)
	c.Assert(err, IsNil)
	c.Assert(cli.config, NotNil)
	c.Check(cli.config.ExchangeTimeout.Duration, Equals, time.Duration(10*time.Millisecond))
}

func (cs *clientSuite) TestConfigureSetsUpLog(c *C) {
	cli := new(Client)
	c.Check(cli.log, IsNil)
	err := cli.Configure(cs.configPath)
	c.Assert(err, IsNil)
	c.Assert(cli.log, NotNil)
}

func (cs *clientSuite) TestConfigureSetsUpPEM(c *C) {
	cli := new(Client)
	c.Check(cli.pem, IsNil)
	err := cli.Configure(cs.configPath)
	c.Assert(err, IsNil)
	c.Assert(cli.pem, NotNil)
}

func (cs *clientSuite) TestReadSetsUpIdder(c *C) {
	cli := new(Client)
	c.Check(cli.idder, IsNil)
	err := cli.Configure(cs.configPath)
	c.Assert(err, IsNil)
	c.Assert(cli.idder, DeepEquals, identifier.New())
}

func (cs *clientSuite) TestConfigureBailsOnBadFilename(c *C) {
	cli := new(Client)
	err := cli.Configure("/does/not/exist")
	c.Assert(err, NotNil)
}

func (cs *clientSuite) TestConfigureBailsOnBadConfig(c *C) {
	cli := new(Client)
	err := cli.Configure("/etc/passwd")
	c.Assert(err, NotNil)
}

func (cs *clientSuite) TestConfigureBailsOnBadPEMFilename(c *C) {
	ioutil.WriteFile(cs.configPath, []byte(`
{
    "exchange_timeout": "10ms",
    "stabilizing_timeout": "0ms",
    "connectivity_check_url": "",
    "connectivity_check_md5": "",
    "addr": ":0",
    "cert_pem_file": "/a/b/c",
    "recheck_timeout": "3h"
}`), 0600)

	cli := new(Client)
	err := cli.Configure(cs.configPath)
	c.Assert(err, NotNil)
}

func (cs *clientSuite) TestConfigureBailsOnBadPEM(c *C) {
	ioutil.WriteFile(cs.configPath, []byte(`
{
    "exchange_timeout": "10ms",
    "stabilizing_timeout": "0ms",
    "connectivity_check_url": "",
    "connectivity_check_md5": "",
    "addr": ":0",
    "cert_pem_file": "/etc/passwd",
    "recheck_timeout": "3h"
}`), 0600)

	cli := new(Client)
	err := cli.Configure(cs.configPath)
	c.Assert(err, NotNil)
}

/*****************************************************************
    getDeviceId tests
******************************************************************/

func (cs *clientSuite) TestGetDeviceIdWorks(c *C) {
	cli := new(Client)
	cli.idder = identifier.New()
	c.Check(cli.deviceId, Equals, "")
	c.Check(cli.getDeviceId(), IsNil)
	c.Check(cli.deviceId, HasLen, 128)
}

func (cs *clientSuite) TestGetDeviceIdCanFail(c *C) {
	cli := new(Client)
	cli.idder = idtesting.Failing()
	c.Check(cli.deviceId, Equals, "")
	c.Check(cli.getDeviceId(), NotNil)
}
