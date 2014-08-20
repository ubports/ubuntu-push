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

package kit

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"launchpad.net/ubuntu-push/config"
)

// MakeTLSConfig makes a tls.Config, optionally reading a cert from
// disk, possibly relative to relDir.
func MakeTLSConfig(domain string, insecure bool, certPEMFile string, relDir string) (*tls.Config, error) {
	tlsConfig := &tls.Config{}
	tlsConfig.ServerName = domain
	tlsConfig.InsecureSkipVerify = insecure
	if !insecure && certPEMFile != "" {
		certPEMBlock, err := config.LoadFile(certPEMFile, relDir)
		if err != nil {
			return nil, fmt.Errorf("reading cert: %v", err)
		}
		cp := x509.NewCertPool()
		ok := cp.AppendCertsFromPEM(certPEMBlock)
		if !ok {
			return nil, fmt.Errorf("could not parse certificate")
		}
		tlsConfig.RootCAs = cp
	}
	return tlsConfig, nil
}
