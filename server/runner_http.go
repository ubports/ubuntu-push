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
	"net/http"

	"launchpad.net/ubuntu-push/config"
)

// A HTTPServeParsedConfig holds and can be used to parse the HTTP server config.
type HTTPServeParsedConfig struct {
	ParsedHTTPAddr         config.ConfigHostPort     `json:"http_addr"`
	ParsedHTTPReadTimeout  config.ConfigTimeDuration `json:"http_read_timeout"`
	ParsedHTTPWriteTimeout config.ConfigTimeDuration `json:"http_write_timeout"`
}

// HTTPServeRunner returns a function to serve HTTP requests.
func HTTPServeRunner(h http.Handler, parsedCfg *HTTPServeParsedConfig) func() {
	httpLst, err := net.Listen("tcp", parsedCfg.ParsedHTTPAddr.HostPort())
	if err != nil {
		BootLogFatalf("start http listening: %v", err)
	}
	BootLogListener("http", httpLst)
	srv := &http.Server{
		Handler:      h,
		ReadTimeout:  parsedCfg.ParsedHTTPReadTimeout.TimeDuration(),
		WriteTimeout: parsedCfg.ParsedHTTPWriteTimeout.TimeDuration(),
	}
	return func() {
		err := srv.Serve(httpLst)
		if err != nil {
			BootLogFatalf("accepting http connections: %v", err)
		}
	}
}
