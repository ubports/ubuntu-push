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

package acceptance_test

import (
	"flag"
	"fmt"
	"testing"

	. "launchpad.net/gocheck"

	"launchpad.net/ubuntu-push/server/acceptance/suites"
)

func TestAcceptance(t *testing.T) { TestingT(t) }

var serverCmd = flag.String("server", "", "server to test")

func testServerConfig(addr, httpAddr string) map[string]interface{} {
	cfg := make(map[string]interface{})
	suites.FillServerConfig(cfg, addr)
	suites.FillHTTPServerConfig(cfg, httpAddr)
	cfg["delivery_domain"] = "localhost"
	return cfg
}

// Start a server.
func StartServer(c *C, s *suites.AcceptanceSuite, handle *suites.ServerHandle) {
	if *serverCmd == "" {
		c.Skip("executable server not specified")
	}
	tmpDir := c.MkDir()
	cfg := testServerConfig("127.0.0.1:0", "127.0.0.1:0")
	cfgFilename := suites.WriteConfig(c, tmpDir, "config.json", cfg)
	logs, killServer := suites.RunAndObserve(c, *serverCmd, cfgFilename)
	s.KillGroup["server"] = killServer
	handle.ServerHTTPAddr = suites.ExtractListeningAddr(c, logs, suites.HTTPListeningOnPat)
	s.ServerAPIURL = fmt.Sprintf("http://%s", handle.ServerHTTPAddr)
	handle.ServerAddr = suites.ExtractListeningAddr(c, logs, suites.DevListeningOnPat)
	handle.ServerEvents = logs
}

// ping pong/connectivity
var _ = Suite(&suites.PingPongAcceptanceSuite{suites.AcceptanceSuite{StartServer: StartServer}})

// broadcast
var _ = Suite(&suites.BroadcastAcceptanceSuite{suites.AcceptanceSuite{StartServer: StartServer}})

// unicast
var _ = Suite(&suites.UnicastAcceptanceSuite{suites.AcceptanceSuite{StartServer: StartServer}, nil})
