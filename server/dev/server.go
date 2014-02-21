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

// simple development server.
package main

import (
	"net"
	"net/http"
	"os"
	"path/filepath"

	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/server"
	"launchpad.net/ubuntu-push/server/api"
	"launchpad.net/ubuntu-push/server/broker/simple"
	"launchpad.net/ubuntu-push/server/session"
	"launchpad.net/ubuntu-push/server/store"
)

type configuration struct {
	// device server configuration
	server.DevicesParsedConfig
	// api http server configuration
	server.HTTPServeParsedConfig
}

func main() {
	cfgFpaths := os.Args[1:]
	cfg := &configuration{}
	err := config.ReadFiles(cfg, cfgFpaths...)
	if err != nil {
		server.BootLogFatalf("reading config: %v", err)
	}
	err = cfg.DevicesParsedConfig.FinishLoad(filepath.Dir(cfgFpaths[len(cfgFpaths)-1]))
	if err != nil {
		server.BootLogFatalf("reading config: %v", err)
	}
	logger := logger.NewSimpleLogger(os.Stderr, "debug")
	// setup a pending store and start the broker
	sto := store.NewInMemoryPendingStore()
	broker := simple.NewSimpleBroker(sto, cfg, logger)
	broker.Start()
	defer broker.Stop()
	// serve the http api
	storeForRequest := func(http.ResponseWriter, *http.Request) (store.PendingStore, error) {
		return sto, nil
	}
	mux := api.MakeHandlersMux(storeForRequest, broker, logger)
	handler := api.PanicTo500Handler(mux, logger)
	go server.HTTPServeRunner(handler, &cfg.HTTPServeParsedConfig)()
	// listen for device connections
	server.DevicesRunner(func(conn net.Conn) error {
		track := session.NewTracker(logger)
		return session.Session(conn, broker, cfg, track)
	}, logger, &cfg.DevicesParsedConfig)()
}
