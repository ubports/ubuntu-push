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
	"crypto/tls"
	"fmt"

	"launchpad.net/ubuntu-push/config"
)

// A TLSParsedConfig holds and can be used to parse a tls server config.
type TLSParsedConfig struct {
	ParsedKeyPEMFile  string `json:"key_pem_file"`
	ParsedCertPEMFile string `json:"cert_pem_file"`
	// private post-processed config
	cert tls.Certificate
}

func (cfg *TLSParsedConfig) LoadPEMs(baseDir string) error {
	keyPEMBlock, err := config.LoadFile(cfg.ParsedKeyPEMFile, baseDir)
	if err != nil {
		return fmt.Errorf("reading key_pem_file: %v", err)
	}
	certPEMBlock, err := config.LoadFile(cfg.ParsedCertPEMFile, baseDir)
	if err != nil {
		return fmt.Errorf("reading cert_pem_file: %v", err)
	}
	cfg.cert, err = tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	return err
}

func (cfg *TLSParsedConfig) TLSServerConfig() *tls.Config {
	tlsCfg := &tls.Config{
		Certificates:           []tls.Certificate{cfg.cert},
		SessionTicketsDisabled: true,
		// order from crypto/tls/cipher_suites.go, no RC4
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
		MinVersion: tls.VersionTLS12,
	}
	fmt.Printf("TLS: %#v\n", tlsCfg)
	return tlsCfg
}
