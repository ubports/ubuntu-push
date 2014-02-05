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

package connectivity

import (
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/logger"
	helpers "launchpad.net/ubuntu-push/testing"
	"launchpad.net/ubuntu-push/util"
	"net/http"
	"net/http/httptest"
	"time"
)

type WebcheckerSuite struct {
	timeouts []time.Duration
	log      logger.Logger
}

var _ = Suite(&WebcheckerSuite{})

const (
	staticText = "something ipsum dolor something"
	staticHash = "6155f83b471583f47c99998a472a178f"
	bigText    = `Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 Vivamus tincidunt vitae sapien tempus fermentum. Cras commodo augue luctu,
 tempus libero sit amet, laoreet lectus. Vestibulum ali justo et malesuada
 placerat. Pellentesque viverra luctus velit, adipiscing fermentum tortori
 vehicula nec. Integer tincidunt purus et pretium vestibulum. Donec portas
 suscipit pulvinar. Suspendisse potenti. Donec sit amet pharetra nisl, sit
 amet posuere orci. In feugiat elitist nec augue fringilla, a rutrum risus
 posuere. Aliquam erat volutpat. Morbi aliquam arcu et eleifend placeraten.
 Pellentesque egestas varius aliquam. In egestas nisi sed ipsum tristiquer
 lacinia. Sed vitae nisi non eros consectetur vestibulum vehicularum vitae.
 Curabitur cursus consectetur eros, in vestibulum turpis cursus at i lorem.
 Pellentesque ultrices arcu ut massa faucibus, e consequat sapien placerat.
 Maecenas quis ultricies mi. Phasellus turpis nisl, porttitor ac mi cursus,
 euismod imperdiet lorem. Donec facilisis est id dignissim imperdiet.`
	bigHash = "9bf86bce26e8f2d9c9d9bd4a98f9e668"
)

// mkHandler makes an http.HandlerFunc that returns the provided text
// for whatever request it's given.
func mkHandler(text string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		w.Write([]byte(text))
		w.(http.Flusher).Flush()
	}
}

func (s *WebcheckerSuite) SetUpSuite(c *C) {
	s.timeouts = util.SwapTimeouts([]time.Duration{0})
}

func (s *WebcheckerSuite) TearDownSuite(c *C) {
	util.SwapTimeouts(s.timeouts)
	s.timeouts = nil
}

func (s *WebcheckerSuite) SetUpTest(c *C) {
	s.log = helpers.NewTestLogger(c, "debug")
}

// Webchecker sends true when everything works
func (s *WebcheckerSuite) TestWorks(c *C) {
	ts := httptest.NewServer(mkHandler(staticText))
	defer ts.Close()

	ck := NewWebchecker(ts.URL, staticHash, s.log)
	ch := make(chan bool, 1)
	ck.Webcheck(ch)
	c.Check(<-ch, Equals, true)
}

// Webchecker sends false if the download fails.
func (s *WebcheckerSuite) TestActualFails(c *C) {
	ck := NewWebchecker("garbage://", "", s.log)
	ch := make(chan bool, 1)
	ck.Webcheck(ch)
	c.Check(<-ch, Equals, false)
}

// Webchecker sends false if the hash doesn't match
func (s *WebcheckerSuite) TestHashFails(c *C) {
	ts := httptest.NewServer(mkHandler(""))
	defer ts.Close()

	ck := NewWebchecker(ts.URL, staticHash, s.log)
	ch := make(chan bool, 1)
	ck.Webcheck(ch)
	c.Check(<-ch, Equals, false)
}

// Webchecker sends false if the download is too big
func (s *WebcheckerSuite) TestTooBigFails(c *C) {
	ts := httptest.NewServer(mkHandler(bigText))
	defer ts.Close()

	ck := NewWebchecker(ts.URL, bigHash, s.log)
	ch := make(chan bool, 1)
	ck.Webcheck(ch)
	c.Check(<-ch, Equals, false)
}
