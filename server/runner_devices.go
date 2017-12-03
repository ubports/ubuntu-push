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

package server

import (
	"net"
	"syscall"
	"time"

	"github.com/ubports/ubuntu-push/config"
	"github.com/ubports/ubuntu-push/logger"
	"github.com/ubports/ubuntu-push/server/listener"
)

// A DevicesParsedConfig holds and can be used to parse the device server config.
type DevicesParsedConfig struct {
	// session configuration
	ParsedPingInterval    config.ConfigTimeDuration `json:"ping_interval"`
	ParsedExchangeTimeout config.ConfigTimeDuration `json:"exchange_timeout"`
	// broker configuration
	ParsedSessionQueueSize config.ConfigQueueSize `json:"session_queue_size"`
	ParsedBrokerQueueSize  config.ConfigQueueSize `json:"broker_queue_size"`
	// device listener configuration
	ParsedAddr config.ConfigHostPort `json:"addr"`
	TLSParsedConfig
}

func (cfg *DevicesParsedConfig) PingInterval() time.Duration {
	return cfg.ParsedPingInterval.TimeDuration()
}

func (cfg *DevicesParsedConfig) ExchangeTimeout() time.Duration {
	return cfg.ParsedExchangeTimeout.TimeDuration()
}

func (cfg *DevicesParsedConfig) SessionQueueSize() uint {
	return cfg.ParsedSessionQueueSize.QueueSize()
}

func (cfg *DevicesParsedConfig) BrokerQueueSize() uint {
	return cfg.ParsedBrokerQueueSize.QueueSize()
}

func (cfg *DevicesParsedConfig) Addr() string {
	return cfg.ParsedAddr.HostPort()
}

// DevicesRunner returns a function to accept device connections.
// If adoptLst is not nil it will be used as the underlying listener, instead
// of creating one, wrapped in a TLS layer.
func DevicesRunner(adoptLst net.Listener, session func(net.Conn) error, logger logger.Logger, resource listener.SessionResourceManager, parsedCfg *DevicesParsedConfig) func() {
	BootLogger.Debugf("PingInterval: %s, ExchangeTimeout %s", parsedCfg.PingInterval(), parsedCfg.ExchangeTimeout())
	var rlim syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlim)
	if err != nil {
		BootLogFatalf("getrlimit failed: %v", err)
	}
	BootLogger.Debugf("nofile soft: %d hard: %d", rlim.Cur, rlim.Max)
	lst, err := listener.DeviceListen(adoptLst, parsedCfg)
	if err != nil {
		BootLogFatalf("start device listening: %v", err)
	}
	BootLogListener("devices", lst)
	return func() {
		err = lst.AcceptLoop(session, resource, logger)
		if err != nil {
			BootLogFatalf("accepting device connections: %v", err)
		}
	}
}
