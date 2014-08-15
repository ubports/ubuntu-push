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

package kit

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/server/acceptance"
)

var (
	insecureFlag    = flag.Bool("insecure", false, "disable checking of server certificate and hostname")
	reportPingsFlag = flag.Bool("reportPings", true, "report each Ping from the server")
	deviceModel     = flag.String("model", "?", "device image model")
	imageChannel    = flag.String("imageChannel", "?", "image channel")
)

type Configuration struct {
	// session configuration
	ExchangeTimeout config.ConfigTimeDuration `json:"exchange_timeout"`
	// server connection config
	Target      string                    `json:"target"`
	Addr        config.ConfigHostPort     `json:"addr"`
	CertPEMFile string                    `json:"cert_pem_file"`
	RunTimeout  config.ConfigTimeDuration `json:"run_timeout"`
}

// Control.
var (
	Name     = "acceptanceclient"
	Defaults = map[string]interface{}{
		"target":           "",
		"addr":             ":0",
		"exchange_timeout": "5s",
		"cert_pem_file":    "",
		"run_timeout":      "0s",
	}
)

// CliLoop parses command line arguments and runs a client loop.
func CliLoop(totalCfg interface{}, cfg *Configuration, onSetup func(*acceptance.ClientSession), auth func(string) string, waitFor func() string, onConnect func()) {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <device id>\n", Name)
		flag.PrintDefaults()
	}
	missingArg := func(what string) {
		fmt.Fprintf(os.Stderr, "missing %s\n", what)
		flag.Usage()
		os.Exit(2)
	}
	err := config.ReadFilesDefaults(totalCfg, Defaults, "<flags>")
	if err != nil {
		log.Fatalf("reading config: %v", err)
	}
	narg := flag.NArg()
	switch {
	case narg < 1:
		missingArg("device-id")
	}
	addr := ""
	if cfg.Addr == ":0" {
		switch cfg.Target {
		case "production":
			addr = "push-delivery.ubuntu.com:443"
		case "staging":
			addr = "push-delivery.staging.ubuntu.com:443"
		case "":
			log.Fatalf("either addr or target must be given")
		default:
			log.Fatalf("if specified target should be prodution|staging")
		}
	} else {
		addr = cfg.Addr.HostPort()
	}
	session := &acceptance.ClientSession{
		ExchangeTimeout: cfg.ExchangeTimeout.TimeDuration(),
		ServerAddr:      addr,
		DeviceId:        flag.Arg(0),
		// flags
		Model:        *deviceModel,
		ImageChannel: *imageChannel,
		ReportPings:  *reportPingsFlag,
		Insecure:     *insecureFlag,
	}
	onSetup(session)
	if !*insecureFlag && cfg.CertPEMFile != "" {
		cfgDir := filepath.Dir(flag.Lookup("cfg@").Value.String())
		log.Printf("cert: %v relToDir: %v", cfg.CertPEMFile, cfgDir)
		session.CertPEMBlock, err = config.LoadFile(cfg.CertPEMFile, cfgDir)
		if err != nil {
			log.Fatalf("reading CertPEMFile: %v", err)
		}
	}
	session.Auth = auth("https://push.ubuntu.com/")
	var waitForRegexp *regexp.Regexp
	waitForStr := waitFor()
	if waitForStr != "" {
		var err error
		waitForRegexp, err = regexp.Compile(waitForStr)
		if err != nil {
			log.Fatalf("wait_for regexp: %v", err)
		}
	}
	err = session.Dial()
	if err != nil {
		log.Fatalln(err)
	}
	events := make(chan string, 5)
	go func() {
		for {
			ev := <-events
			if strings.HasPrefix(ev, "connected") {
				onConnect()
			}
			if waitForRegexp != nil && waitForRegexp.MatchString(ev) {
				log.Println("<matching-event>:", ev)
				os.Exit(0)
			}
			log.Println(ev)
		}
	}()
	if cfg.RunTimeout.TimeDuration() != 0 {
		time.AfterFunc(cfg.RunTimeout.TimeDuration(), func() {
			log.Fatalln("<run timed out>")
		})
	}
	err = session.Run(events)
	if err != nil {
		log.Fatalln(err)
	}
}
