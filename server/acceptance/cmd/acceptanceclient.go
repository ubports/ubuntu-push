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

// acceptanceclient command for playing.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/server/acceptance"
)

var (
	insecureFlag    = flag.Bool("insecure", false, "disable checking of server certificate and hostname")
	reportPingsFlag = flag.Bool("reportPings", true, "report each Ping from the server")
	deviceModel     = flag.String("model", "?", "device image model")
	imageChannel    = flag.String("imageChannel", "?", "image channel")
)

type configuration struct {
	// session configuration
	ExchangeTimeout config.ConfigTimeDuration `json:"exchange_timeout"`
	// server connection config
	Addr        config.ConfigHostPort
	CertPEMFile string `json:"cert_pem_file"`
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: acceptancclient [options] <config.json> <device id>\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	narg := flag.NArg()
	switch {
	case narg < 1:
		log.Fatal("missing config file")
	case narg < 2:
		log.Fatal("missing device-id")
	}
	configFName := flag.Arg(0)
	f, err := os.Open(configFName)
	if err != nil {
		log.Fatalf("reading config: %v", err)
	}
	cfg := &configuration{}
	err = config.ReadConfig(f, cfg)
	if err != nil {
		log.Fatalf("reading config: %v", err)
	}
	session := &acceptance.ClientSession{
		ExchangeTimeout: cfg.ExchangeTimeout.TimeDuration(),
		ServerAddr:      cfg.Addr.HostPort(),
		DeviceId:        flag.Arg(1),
		// flags
		Model:        *deviceModel,
		ImageChannel: *imageChannel,
		ReportPings:  *reportPingsFlag,
		Insecure:     *insecureFlag,
	}
	log.Printf("with: %#v", session)
	if !*insecureFlag {
		session.CertPEMBlock, err = config.LoadFile(cfg.CertPEMFile, filepath.Dir(configFName))
		if err != nil {
			log.Fatalf("reading CertPEMFile: %v", err)
		}
	}
	err = session.Dial()
	if err != nil {
		log.Fatalln(err)
	}
	events := make(chan string, 5)
	go func() {
		for {
			log.Println(<-events)
		}
	}()
	err = session.Run(events)
	if err != nil {
		log.Fatalln(err)
	}
}
