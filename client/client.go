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

// The client package implements the Ubuntu Push Notifications client-side
// daemon.
package client

import (
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"launchpad.net/ubuntu-push/bus/connectivity"
	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/whoopsie/identifier"
	"os"
)

type ClientConfig struct {
	connectivity.ConnectivityConfig
	// session configuration
	ExchangeTimeout config.ConfigTimeDuration `json:"exchange_timeout"`
	// server connection config
	Addr        config.ConfigHostPort
	CertPEMFile string `json:"cert_pem_file"`
}

type Client struct {
	config   ClientConfig
	log      logger.Logger
	pem      []byte
	idder    identifier.Id
	deviceId string
}

// Configure loads the configuration specified in configPath, and sets it up.
func (client *Client) Configure(configPath string) error {
	f, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("opening config: %v", err)
	}
	err = config.ReadConfig(f, &client.config)
	if err != nil {
		return fmt.Errorf("reading config: %v", err)
	}
	// later, we'll be specifying logging options in the config file
	client.log = logger.NewSimpleLogger(os.Stderr, "error")

	// overridden for testing
	client.idder = identifier.New()

	if client.config.CertPEMFile != "" {
		client.pem, err = ioutil.ReadFile(client.config.CertPEMFile)
		if err != nil {
			return fmt.Errorf("reading PEM file: %v", err)
		}
		// sanity check
		p, _ := pem.Decode(client.pem)
		if p == nil {
			return fmt.Errorf("no PEM found in PEM file")
		}
	}

	return nil
}

// getDeviceId gets the whoopsie identifier for the device
func (client *Client) getDeviceId() error {
	err := client.idder.Generate()
	if err != nil {
		return err
	}
	client.deviceId = client.idder.String()
	return nil
}
