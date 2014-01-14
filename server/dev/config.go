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
	"fmt"
	"io"
	"launchpad.net/ubuntu-push/config"
	"time"
)

// expectedConfiguration is used as target for JSON parsing the configuration.
type expectedConfiguration struct {
	// session configuration
	PingInterval    config.ConfigTimeDuration `json:"ping_interval"`
	ExchangeTimeout config.ConfigTimeDuration `json:"exchange_timeout"`
	// broker configuration
	SessionQueueSize config.ConfigQueueSize `json:"session_queue_size"`
	BrokerQueueSize  config.ConfigQueueSize `json:"broker_queue_size"`
	// device listener configuration
	Addr        config.ConfigHostPort `json:"addr"`
	KeyPEMFile  string                `json:"key_pem_file"`
	CertPEMFile string                `json:"cert_pem_file"`
	// api http server configuration
	HTTPAddr         config.ConfigHostPort     `json:"http_addr"`
	HTTPReadTimeout  config.ConfigTimeDuration `json:"http_read_timeout"`
	HTTPWriteTimeout config.ConfigTimeDuration `json:"http_write_timeout"`
}

// configuration holds the server configuration and gives it access
// through the various component config interfaces.
type configuration struct {
	parsed       expectedConfiguration
	certPEMBlock []byte
	keyPEMBlock  []byte
}

func (cfg *configuration) PingInterval() time.Duration {
	return cfg.parsed.PingInterval.TimeDuration()
}

func (cfg *configuration) ExchangeTimeout() time.Duration {
	return cfg.parsed.ExchangeTimeout.TimeDuration()
}

func (cfg *configuration) SessionQueueSize() uint {
	return cfg.parsed.SessionQueueSize.QueueSize()
}

func (cfg *configuration) BrokerQueueSize() uint {
	return cfg.parsed.BrokerQueueSize.QueueSize()
}

func (cfg *configuration) Addr() string {
	return cfg.parsed.Addr.HostPort()
}

func (cfg *configuration) KeyPEMBlock() []byte {
	return cfg.keyPEMBlock
}

func (cfg *configuration) CertPEMBlock() []byte {
	return cfg.certPEMBlock
}

func (cfg *configuration) HTTPAddr() string {
	return cfg.parsed.HTTPAddr.HostPort()
}

func (cfg *configuration) HTTPReadTimeout() time.Duration {
	return cfg.parsed.HTTPReadTimeout.TimeDuration()
}

func (cfg *configuration) HTTPWriteTimeout() time.Duration {
	return cfg.parsed.HTTPWriteTimeout.TimeDuration()
}

// read reads & parses configuration from the reader. it uses baseDir
// to load mentioned files in the configuration.
func (cfg *configuration) read(r io.Reader, baseDir string) error {
	err := config.ReadConfig(r, &cfg.parsed)
	if err != nil {
		return err
	}
	keyPEMBlock, err := config.LoadFile(cfg.parsed.KeyPEMFile, baseDir)
	if err != nil {
		return fmt.Errorf("reading key_pem_file: %v", err)
	}
	certPEMBlock, err := config.LoadFile(cfg.parsed.CertPEMFile, baseDir)
	if err != nil {
		return fmt.Errorf("reading cert_pem_file: %v", err)
	}
	cfg.keyPEMBlock = keyPEMBlock
	cfg.certPEMBlock = certPEMBlock
	return nil
}
