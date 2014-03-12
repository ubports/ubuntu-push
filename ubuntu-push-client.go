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

	"launchpad.net/go-xdg/v0"
	"launchpad.net/ubuntu-push/client"
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
	cli := client.NewPushClient(cfgFname, lvlFname)
	err = cli.Start()
	if err != nil {
		log.Fatalf("unable to start: %v", err)
	}
	cli.Loop()
}
