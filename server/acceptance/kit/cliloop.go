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
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"launchpad.net/ubuntu-push/external/murmur3"

	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/server/acceptance"
)

type Configuration struct {
	// session configuration
	ExchangeTimeout config.ConfigTimeDuration `json:"exchange_timeout"`
	// server connection config
	Target      string                `json:"target" help:"production|staging - picks defaults"`
	Addr        config.ConfigHostPort `json:"addr"`
	Vnode       string                `json:"vnode" help:"vnode postfix to make up a targeting device-id"`
	CertPEMFile string                `json:"cert_pem_file"`
	Insecure    bool                  `json:"insecure" help:"disable checking of server certificate and hostname"`
	Domain      string                `json:"domain" help:"domain for tls connect"`
	// api config
	APIURL         string `json:"api" help:"api url"`
	APICertPEMFile string `json:"api_cert_pem_file"`
	// run timeout
	RunTimeout config.ConfigTimeDuration `json:"run_timeout"`
	// flags
	ReportPings  bool   `json:"reportPings" help:"report each Ping from the server"`
	DeviceModel  string `json:"model" help:"device image model"`
	ImageChannel string `json:"imageChannel" help:"image channel"`
	BuildNumber  int32  `json:"buildNumber" help:"build number"`
}

func (cfg *Configuration) PickByTarget(what, productionValue, stagingValue string) (value string) {
	switch cfg.Target {
	case "production":
		value = productionValue
	case "staging":
		value = stagingValue
	case "":
		log.Fatalf("either %s or target must be given", what)
	default:
		log.Fatalf("if specified target should be production|staging")
	}
	return
}

// Control.
var (
	Name     = "acceptanceclient"
	Defaults = map[string]interface{}{
		"target":            "",
		"addr":              ":0",
		"vnode":             "",
		"exchange_timeout":  "5s",
		"cert_pem_file":     "",
		"insecure":          false,
		"domain":            "",
		"run_timeout":       "0s",
		"reportPings":       true,
		"model":             "?",
		"imageChannel":      "?",
		"buildNumber":       -1,
		"api":               "",
		"api_cert_pem_file": "",
	}
)

// CliLoop parses command line arguments and runs a client loop.
func CliLoop(totalCfg interface{}, cfg *Configuration, onSetup func(sess *acceptance.ClientSession, apiCli *APIClient, cfgDir string), auth func(string) string, waitFor func() string, onConnect func()) {
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
	deviceId := ""
	if cfg.Vnode != "" {
		if cfg.Addr == ":0" {
			log.Fatalf("-vnode needs -addr specified")
		}
		deviceId = cfg.Addr.HostPort() + "|" + cfg.Vnode
		log.Printf("using device-id: %q", deviceId)

	} else {
		narg := flag.NArg()
		switch {
		case narg < 1:
			missingArg("device-id")
		}
		deviceId = flag.Arg(0)
	}
	cfgDir := filepath.Dir(flag.Lookup("cfg@").Value.String())
	// setup api
	apiCli := &APIClient{}
	var apiTLSConfig *tls.Config
	if cfg.APICertPEMFile != "" || cfg.Insecure {
		var err error
		apiTLSConfig, err = MakeTLSConfig("", cfg.Insecure,
			cfg.APICertPEMFile, cfgDir)
		if err != nil {
			log.Fatalf("api tls config: %v", err)
		}
	}
	apiCli.SetupClient(apiTLSConfig, true, 1)
	if cfg.APIURL == "" {
		apiCli.ServerAPIURL = cfg.PickByTarget("api",
			"https://push.ubuntu.com",
			"https://push.staging.ubuntu.com")
	} else {
		apiCli.ServerAPIURL = cfg.APIURL
	}
	addr := ""
	domain := ""
	if cfg.Addr == ":0" {
		hash := murmur3.Sum64([]byte(deviceId))
		hosts, err := apiCli.GetRequest("/delivery-hosts",
			map[string]string{
				"h": fmt.Sprintf("%x", hash),
			})
		if err != nil {
			log.Fatalf("querying hosts: %v", err)
		}
		addr = hosts["hosts"].([]interface{})[0].(string)
		domain = hosts["domain"].(string)
		log.Printf("using: %s %s", addr, domain)
	} else {
		addr = cfg.Addr.HostPort()
		domain = cfg.Domain
	}
	session := &acceptance.ClientSession{
		ExchangeTimeout: cfg.ExchangeTimeout.TimeDuration(),
		ServerAddr:      addr,
		DeviceId:        deviceId,
		// flags
		Model:        cfg.DeviceModel,
		ImageChannel: cfg.ImageChannel,
		BuildNumber:  cfg.BuildNumber,
		ReportPings:  cfg.ReportPings,
	}
	onSetup(session, apiCli, cfgDir)
	session.TLSConfig, err = MakeTLSConfig(domain, cfg.Insecure, cfg.CertPEMFile, cfgDir)
	if err != nil {
		log.Fatalf("tls config: %v", err)
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
