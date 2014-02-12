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

package acceptance

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	. "launchpad.net/gocheck"

	helpers "launchpad.net/ubuntu-push/testing"
)

// FillConfig fills cfg from values.
func FillConfig(cfg, values map[string]interface{}) {
	for k, v := range values {
		cfg[k] = v
	}
}

// FillServerConfig fills cfg with default server values and "addr": addr.
func FillServerConfig(cfg map[string]interface{}, addr string) {
	FillConfig(cfg, map[string]interface{}{
		"exchange_timeout":   "0.1s",
		"ping_interval":      "0.5s",
		"session_queue_size": 10,
		"broker_queue_size":  100,
		"addr":               addr,
		"key_pem_file":       helpers.SourceRelative("config/testing.key"),
		"cert_pem_file":      helpers.SourceRelative("config/testing.cert"),
	})
}

// FillHttpServerConfig fills cfg with default http server values and
// "http_addr": httpAddr.
func FillHTTPServerConfig(cfg map[string]interface{}, httpAddr string) {
	FillConfig(cfg, map[string]interface{}{
		"http_addr":          httpAddr,
		"http_read_timeout":  "1s",
		"http_write_timeout": "1s",
	})
}

// WriteConfig writes out a config and returns the written filepath.
func WriteConfig(c *C, dir, filename string, cfg map[string]interface{}) string {
	cfgFpath := filepath.Join(dir, filename)
	cfgJson, err := json.Marshal(cfg)
	if err != nil {
		c.Fatal(err)
	}
	err = ioutil.WriteFile(cfgFpath, cfgJson, os.ModePerm)
	if err != nil {
		c.Fatal(err)
	}
	return cfgFpath
}

var rxLineInfo = regexp.MustCompile("^.*? ([[:alpha:]].*)\n")

// RunAndObserve runs cmdName and returns a channel that will receive
// cmdName stderr logging and a function to kill the process.
func RunAndObserve(c *C, cmdName string, arg ...string) (<-chan string, func()) {
	cmd := exec.Command(cmdName, arg...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		c.Fatal(err)
	}
	err = cmd.Start()
	if err != nil {
		c.Fatal(err)
	}
	bufErr := bufio.NewReaderSize(stderr, 5000)
	getLineInfo := func() (string, error) {
		for {
			line, err := bufErr.ReadString('\n')
			if err != nil {
				return "", err
			}
			extracted := rxLineInfo.FindStringSubmatch(line)
			if extracted == nil {
				return "", fmt.Errorf("unexpected line: %#v", line)
			}
			info := extracted[1]
			return info, nil
		}
	}
	logs := make(chan string, 10)
	go func() {
		for {
			info, err := getLineInfo()
			if err != nil {
				logs <- fmt.Sprintf("%s capture: %v", cmdName, err)
				close(logs)
				return
			}
			logs <- info
		}
	}()
	return logs, func() { cmd.Process.Kill() }
}

const (
	DevListeningOnPat  = "INFO listening for devices on "
	HTTPListeningOnPat = "INFO listening for http on "
	debugPrefix        = "DEBUG "
)

// ExtractListeningAddr goes over logs ignoring DEBUG lines
// until a line starting with pat and returns the rest of that line.
func ExtractListeningAddr(c *C, logs <-chan string, pat string) string {
	for line := range logs {
		if strings.HasPrefix(line, debugPrefix) {
			continue
		}
		if !strings.HasPrefix(line, pat) {
			c.Fatalf("matching %v: %v", pat, line)
		}
		return line[len(pat):]
	}
	panic(fmt.Errorf("logs closed unexpectedly marching %v", pat))
}

// NextEvent receives an event from given string channel with a 5s timeout,
// or from a channel for errors.
func NextEvent(events <-chan string, errCh <-chan error) string {
	select {
	case <-time.After(5 * time.Second):
		panic("too long stuck waiting for next event")
	case err := <-errCh:
		return err.Error() // will fail comparison typically
	case evStr := <-events:
		return evStr
	}
}
