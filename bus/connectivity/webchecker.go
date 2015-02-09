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
	"time"

	http13 "launchpad.net/ubuntu-push/http13client"

	"launchpad.net/ubuntu-push/logger"
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
	cli    *http13.Client
}

// Build a webchecker for the given URL, that should match the target MD5.
func NewWebchecker(url string, target string, timeout time.Duration, log logger.Logger) Webchecker {
	cli := &http13.Client{
		Timeout:   timeout,
		Transport: &http13.Transport{TLSHandshakeTimeout: timeout},
	}
	return &webchecker{log, url, target, cli}
}

// ensure webchecker implements Webchecker
var _ Webchecker = &webchecker{}

func (wb *webchecker) Webcheck(ch chan<- bool) {
	response, err := wb.cli.Get(wb.url)
	if err != nil {
		wb.log.Errorf("while GETting %s: %v", wb.url, err)
		ch <- false
		return
	}
	defer response.Body.Close()
	hash := md5.New()
	_, err = io.CopyN(hash, response.Body, 1024)
	if err != io.EOF {
		if err == nil {
			wb.log.Errorf("reading %s, but response body is larger than 1k.", wb.url)
		} else {
			wb.log.Errorf("reading %s, expecting EOF, got: %v", wb.url, err)
		}
		ch <- false
		return
	}
	sum := fmt.Sprintf("%x", hash.Sum(nil))
	if sum == wb.target {
		wb.log.Infof("connectivity check passed.")
		ch <- true
	} else {
		wb.log.Infof("connectivity check failed: content mismatch.")
		ch <- false
	}
}
