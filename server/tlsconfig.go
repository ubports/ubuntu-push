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
	certPEMBlock []byte
	keyPEMBlock  []byte
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
	cfg.keyPEMBlock = keyPEMBlock
	cfg.certPEMBlock = certPEMBlock
	return nil
}

func (cfg *TLSParsedConfig) TLSServerConfig() (*tls.Config, error) {
	cert, err := tls.X509KeyPair(cfg.certPEMBlock, cfg.keyPEMBlock)
	if err != nil {
		return nil, err
	}
	tlsCfg := &tls.Config{
		Certificates:           []tls.Certificate{cert},
		SessionTicketsDisabled: true,
	}
	return tlsCfg, nil
}
