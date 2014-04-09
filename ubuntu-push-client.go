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

package main

import (
	"log"
	"os"

	"gopkg.in/niemeyer/uoneauth.v1"
	"gopkg.in/qml.v0"

	"launchpad.net/go-xdg/v0"

	"launchpad.net/ubuntu-push/client"
	"launchpad.net/ubuntu-push/logger"
)

func main() {
	cfgFname, err := xdg.Config.Find("ubuntu-push-client/config.json")
	if err != nil {
		log.Fatalf("unable to find a configuration file: %v", err)
	}
	lvlFname, err := xdg.Data.Ensure("ubuntu-push-client/levels.db")
	if err != nil {
		log.Fatalf("unable to open the levels database: %v", err)
	}
	authLogger := logger.NewSimpleLogger(os.Stderr, "debug")
	qml.SetLogger(authLogger)
	qml.Init(nil)
	engine := qml.NewEngine()
	defer engine.Destroy()
	authService := uoneauth.NewService(engine)
	var auth string
	token, err := authService.Token()
	if err != nil {
		authLogger.Errorf("unable to get the authorization token from the account: %v", err)
	} else {
		auth = token.HeaderSignature("POST", "https://push.ubuntu.com")
	}

	cli := client.NewPushClient(cfgFname, lvlFname, auth)
	err = cli.Start()
	if err != nil {
		log.Fatalf("unable to start: %v", err)
	}
	cli.Loop()
}
