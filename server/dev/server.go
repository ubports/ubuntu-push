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
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/server/api"
	"launchpad.net/ubuntu-push/server/broker"
	"launchpad.net/ubuntu-push/server/listener"
	"launchpad.net/ubuntu-push/server/session"
	"launchpad.net/ubuntu-push/server/store"
	"net"
	"os"
	"path/filepath"
)

func main() {
	logger := logger.NewSimpleLogger(os.Stderr, "debug")
	if len(os.Args) < 2 { // xxx use flag
		logger.Fatalf("missing config file")
	}
	configFName := os.Args[1]
	f, err := os.Open(configFName)
	if err != nil {
		logger.Fatalf("reading config: %v", err)
	}
	cfg := &configuration{}
	err = cfg.read(f, filepath.Dir(configFName))
	if err != nil {
		logger.Fatalf("reading config: %v", err)
	}
	// setup a pending store and start the broker
	sto := store.NewInMemoryPendingStore()
	broker := broker.NewSimpleBroker(sto, cfg, logger)
	broker.Start()
	defer broker.Stop()
	// serve the http api
	httpLst, err := net.Listen("tcp", cfg.HTTPAddr())
	if err != nil {
		logger.Fatalf("start http listening: %v", err)
	}
	handler := api.MakeHandlersMux(sto, broker, logger)
	logger.Infof("listening for http on %v", httpLst.Addr())
	go func() {
		err := RunHTTPServe(httpLst, handler, cfg)
		if err != nil {
			logger.Fatalf("accepting http connections: %v", err)
		}
	}()
	// listen for device connections
	lst, err := listener.DeviceListen(cfg)
	if err != nil {
		logger.Fatalf("start device listening: %v", err)
	}
	logger.Infof("listening for devices on %v", lst.Addr())
	err = lst.AcceptLoop(func(conn net.Conn) error {
		track := session.NewTracker(logger)
		return session.Session(conn, broker, cfg, track)
	}, logger)
	if err != nil {
		logger.Fatalf("accepting device connections: %v", err)
	}
}
