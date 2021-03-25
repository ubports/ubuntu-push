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

// Dev is a simple development server.
package main

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ubports/ubuntu-push/config"
	"github.com/ubports/ubuntu-push/logger"
	"github.com/ubports/ubuntu-push/server"
	"github.com/ubports/ubuntu-push/server/api"
	"github.com/ubports/ubuntu-push/server/broker/simple"
	"github.com/ubports/ubuntu-push/server/listener"
	"github.com/ubports/ubuntu-push/server/session"
	"github.com/ubports/ubuntu-push/server/statistics"
	"github.com/ubports/ubuntu-push/server/store"
)

type configuration struct {
	// device server configuration
	server.DevicesParsedConfig
	// api http server configuration
	server.HTTPServeParsedConfig
	// user credentials to be used for access to the stats endpoint
	StatisticsAuthUser string `json:"statistics_auth_user"`
	// password credentials to be used for access to the stats endpoint
	StatisticsAuthPassword string `json:"statistics_auth_password"`
	// delivery domain
	DeliveryDomain string `json:"delivery_domain"`
	// max notifications per application
	MaxNotificationsPerApplication int `json:"max_notifications_per_app"`
}

type Storage struct {
	sto                            store.PendingStore
	maxNotificationsPerApplication int
}

func (storage *Storage) StoreForRequest(http.ResponseWriter, *http.Request) (store.PendingStore, error) {
	return storage.sto, nil
}

func (storage *Storage) GetMaxNotificationsPerApplication() int {
	return storage.maxNotificationsPerApplication
}

func main() {
	cfgFpaths := os.Args[1:]
	cfg := &configuration{}
	err := config.ReadFiles(cfg, cfgFpaths...)
	if err != nil {
		server.BootLogFatalf("reading config: %v", err)
	}
	err = cfg.DevicesParsedConfig.LoadPEMs(filepath.Dir(cfgFpaths[len(cfgFpaths)-1]))
	if err != nil {
		server.BootLogFatalf("reading config: %v", err)
	}
	logger := logger.NewSimpleLogger(os.Stderr, "info")
	// Setup statistics
	currentStats := statistics.NewStatistics(logger)
	// setup a pending store and start the broker
	sto := store.NewInMemoryPendingStore()
	broker := simple.NewSimpleBroker(sto, cfg, logger, currentStats)
	broker.Start()
	defer broker.Stop()
	// serve the http api
	storage := &Storage{
		sto:                            sto,
		maxNotificationsPerApplication: cfg.MaxNotificationsPerApplication,
	}
	lst, err := net.Listen("tcp", cfg.Addr())
	if err != nil {
		server.BootLogFatalf("start device listening: %v", err)
	}
	mux := api.MakeHandlersMux(storage, broker, logger)
	// & /delivery-hosts
	mux.HandleFunc("/delivery-hosts", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.Encode(map[string]interface{}{
			"hosts":  []string{lst.Addr().String()},
			"domain": cfg.DeliveryDomain,
		})
	})
	// /stats
	mux.HandleFunc("/stats", func(w http.ResponseWriter, req *http.Request) {
		var err error
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		s := strings.SplitN(req.Header.Get("Authorization"), " ", 2)
		if len(s) != 2 {
			http.Error(w, "Not authorized", 401)
			return
		}

		b, err := base64.StdEncoding.DecodeString(s[1])
		if err != nil {
			http.Error(w, err.Error(), 401)
			return
		}

		pair := strings.SplitN(string(b), ":", 2)
		if len(pair) != 2 {
			http.Error(w, "Not authorized", 401)
			return
		}

		if pair[0] != cfg.StatisticsAuthUser || pair[1] != cfg.StatisticsAuthPassword {
			http.Error(w, "Not authorized", 401)
			return
		}

		statsData := currentStats.GetStats()
		statsJSON := &[]byte{}
		*statsJSON, err = json.Marshal(statsData)
		if err != nil {
			server.BootLogFatalf("Error marshal stats json: %v", err)
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(*statsJSON)
	})
	handler := api.PanicTo500Handler(mux, logger)
	go server.HTTPServeRunner(nil, handler, &cfg.HTTPServeParsedConfig, cfg.DevicesParsedConfig.TLSServerConfig())()
	// listen for device connections
	resource := &listener.NopSessionResourceManager{}
	server.DevicesRunner(lst, func(conn net.Conn) error {
		track := session.NewTracker(logger)
		return session.Session(conn, broker, cfg, track)
	}, logger, resource, &cfg.DevicesParsedConfig)()
}
