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
	"net"
	"net/http"
	"golang.org/x/net/netutil"

	"github.com/ubports/ubuntu-push/config"
)

// A HTTPServeParsedConfig holds and can be used to parse the HTTP server config.
type HTTPServeParsedConfig struct {
	ParsedHTTPAddr         config.ConfigHostPort     `json:"http_addr"`
	ParsedHTTPReadTimeout  config.ConfigTimeDuration `json:"http_read_timeout"`
	ParsedHTTPWriteTimeout config.ConfigTimeDuration `json:"http_write_timeout"`
}

// HTTPServeRunner returns a function to serve HTTP requests.
// If httpLst is not nil it will be used as the underlying listener.
// If tlsCfg is not nit server over TLS with the config.
func HTTPServeRunner(httpLst net.Listener, h http.Handler, parsedCfg *HTTPServeParsedConfig, tlsCfg *tls.Config) func() {
	if httpLst == nil {
		var err error
		httpLst, err = net.Listen("tcp", parsedCfg.ParsedHTTPAddr.HostPort())
		if err != nil {
			BootLogFatalf("start http listening: %v", err)
		}
		httpLst = netutil.LimitListener(httpLst, 200)
	}
	BootLogListener("http", httpLst)
	srv := &http.Server{
		Handler:      h,
		ReadTimeout:  parsedCfg.ParsedHTTPReadTimeout.TimeDuration(),
		WriteTimeout: parsedCfg.ParsedHTTPWriteTimeout.TimeDuration(),
	}
	if tlsCfg != nil {
		httpLst = tls.NewListener(httpLst, tlsCfg)
		httpLst = netutil.LimitListener(httpLst, 100)
	}
	return func() {
		err := srv.Serve(httpLst)
		if err != nil {
			BootLogFatalf("accepting http connections: %v", err)
		}
	}
}
