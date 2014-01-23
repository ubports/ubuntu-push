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

// webchecker checks whether we're actually connected by doing an http
// GET to the Ubuntu connectivity check URL,
// http://start.ubuntu.com/connectivity-check.html
//
// We could make it be https to make extra doubly sure, but it's expensive
// overkill for the majority of cases.
package connectivity

import (
	"crypto/md5"
	"fmt"
	"io"
	"launchpad.net/ubuntu-push/logger"
	"net/http"
)

// how much web would a webchecker check

type Webchecker interface {
	// Webcheck checks whether retrieving the URL works, and if its
	// contents match the target. If so, then it sends true; if anything
	// fails, it sends false.
	Webcheck(chan<- bool)
}

type webchecker struct {
	log    logger.Logger
	url    string
	target string
}

// Build a webchecker for the given URL, that should match the target MD5.
func NewWebchecker(url string, target string, log logger.Logger) Webchecker {
	return &webchecker{log, url, target}
}

// ensure webchecker implements Webchecker
var _ Webchecker = &webchecker{}

func (wb *webchecker) Webcheck(ch chan<- bool) {
	response, err := http.Get(wb.url)
	if err != nil {
		wb.log.Errorf("While GETting %s: %s", wb.url, err)
		ch <- false
		return
	}
	defer response.Body.Close()
	hash := md5.New()
	_, err = io.CopyN(hash, response.Body, 1024)
	if err != io.EOF {
		wb.log.Errorf("Reading %s, expecting EOF, got: %s",
			wb.url, err)
		ch <- false
		return
	}
	sum := fmt.Sprintf("%x", hash.Sum(nil))
	if sum == wb.target {
		wb.log.Infof("Connectivity check passed.")
		ch <- true
	} else {
		wb.log.Infof("Connectivity check failed: content mismatch.")
		ch <- false
	}
}
