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

// Package listener has code to listen for device connections and setup sessions for them.
package listener

import (
	"crypto/tls"
	"launchpad.net/ubuntu-push/logger"
	"net"
	"time"
)

// A DeviceListenerConfig offers the DeviceListener configuration.
type DeviceListenerConfig interface {
	// Addr to listen on.
	Addr() string
	// TLS key
	KeyPEMBlock() []byte
	// TLS cert
	CertPEMBlock() []byte
}

// DeviceListener listens and setup sessions from device connections.
type DeviceListener struct {
	net.Listener
}

// DeviceListen creates a DeviceListener for device connections based on config.
func DeviceListen(cfg DeviceListenerConfig) (*DeviceListener, error) {
	cert, err := tls.X509KeyPair(cfg.CertPEMBlock(), cfg.KeyPEMBlock())
	if err != nil {
		return nil, err
	}
	tlsCfg := &tls.Config{
		Certificates:           []tls.Certificate{cert},
		SessionTicketsDisabled: true,
	}
	lst, err := tls.Listen("tcp", cfg.Addr(), tlsCfg)
	return &DeviceListener{lst}, err
}

// handleTemporary checks and handles if the error is just a temporary network
// error.
func handleTemporary(err error) bool {
	if netError, isNetError := err.(net.Error); isNetError {
		if netError.Temporary() {
			// wait, xxx exponential backoff?
			time.Sleep(100 * time.Millisecond)
			return true
		}
	}
	return false
}

// AcceptLoop accepts connections and starts sessions for them.
func (dl *DeviceListener) AcceptLoop(session func(net.Conn) error, logger logger.Logger) error {
	for {
		// xxx enforce a connection limit
		conn, err := dl.Listener.Accept()
		if err != nil {
			if handleTemporary(err) {
				logger.Errorf("device listener: %s -- retrying", err)
				continue
			}
			return err
		}
		go func() {
			defer func() {
				if err := recover(); err != nil {
					logger.PanicStackf("terminating device connection on: %v", err)
				}
			}()
			session(conn)
		}()
	}
}
