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

// acceptanceclient command for playing.
package main

import (
	"log"
	"os/exec"
	"strings"

	"github.com/ubports/ubuntu-push/server/acceptance"
	"github.com/ubports/ubuntu-push/server/acceptance/kit"
)

type configuration struct {
	kit.Configuration
	AuthHelper string `json:"auth_helper"`
	WaitFor    string `json:"wait_for"`
}

func main() {
	kit.Defaults["auth_helper"] = ""
	kit.Defaults["wait_for"] = ""
	cfg := &configuration{}
	kit.CliLoop(cfg, &cfg.Configuration, func(session *acceptance.ClientSession, apiCli *kit.APIClient, cfgDir string) {
		log.Printf("with: %#v", session)
	}, func(url string) string {
		if cfg.AuthHelper == "" {
			return ""
		}
		auth, err := exec.Command(cfg.AuthHelper, url).Output()
		if err != nil {
			log.Fatalf("auth helper: %v", err)
		}
		return strings.TrimSpace(string(auth))
	}, func() string {
		return cfg.WaitFor
	}, func() {
	})
}
