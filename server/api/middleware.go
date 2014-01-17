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

package api

import (
	"fmt"
	"launchpad.net/ubuntu-push/logger"
	"net/http"
)

// PanicTo500Handler wraps another handler such that panics are recovered
// and 500 reported.
func PanicTo500Handler(h http.Handler, logger logger.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.PanicStackf("serving http: %v", err)
				// best effort
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(500)
				fmt.Fprintf(w, `{"error":"internal","message":"INTERNAL SERVER ERROR"}`)
			}
		}()
		h.ServeHTTP(w, req)
	})
}
