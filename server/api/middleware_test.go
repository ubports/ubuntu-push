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
	"net/http"
	"net/http/httptest"

	. "launchpad.net/gocheck"

	helpers "github.com/ubports/ubuntu-push/testing"
)

type middlewareSuite struct{}

var _ = Suite(&middlewareSuite{})

func (s *middlewareSuite) TestPanicTo500Handler(c *C) {
	logger := helpers.NewTestLogger(c, "debug")
	panicking := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		panic("panic in handler")
	})

	h := PanicTo500Handler(panicking, logger)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, nil)
	c.Check(w.Code, Equals, 500)
	c.Check(logger.Captured(), Matches, "(?s)ERROR\\(PANIC\\) serving http: panic in handler:.*")
	c.Check(w.Header().Get("Content-Type"), Equals, "application/json")
	c.Check(w.Body.String(), Equals, `{"error":"internal","message":"INTERNAL SERVER ERROR"}`)
}
