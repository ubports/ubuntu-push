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

// a silly example of the connectivity api
package main

import (
	"fmt"
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/connectivity"
	"launchpad.net/ubuntu-push/logger"
	"os"
	"strings"
)

func main() {
	log := logger.NewSimpleLogger(os.Stderr, "error")

	paths := []string{"thing.json", "connectivity/example/thing.json"}
	for _, path := range paths {
		cff, err := os.Open(path)
		if err == nil {
			var cfg connectivity.Config
			err = config.ReadConfig(cff, &cfg)
			if err != nil {
				log.Fatalf("%s", err)
			}

			ch := make(chan bool)
			go connectivity.ConnectedState(bus.SystemBus, cfg, log, ch)

			for c := range ch {
				fmt.Println("Are we connected?", c)
			}
			return
		}
	}
	log.Fatalf("Unable to open the config file; tried %s.", strings.Join(paths, ", "))

}
