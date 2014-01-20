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

// Package connectivity a single, simple stream of booleans to answer
// the quesiton “are we connected?”.
//
// It can potentially fire two falses in a row, if a disconnected
// state is followed by a dbus watch error. Other than that, it's edge
// triggered.
package connectivity

import (
	"errors"
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/networkmanager"
	"launchpad.net/ubuntu-push/connectivity/webchecker"
	"launchpad.net/ubuntu-push/logger"
	"time"
)

type Config struct {
	ConnectTimeouts      []config.ConfigTimeDuration
	StabilizingTimeout   config.ConfigTimeDuration
	RecheckTimeout       config.ConfigTimeDuration
	ConnectivityCheckURL string
	ConnectivityCheckMD5 string
}

type connectedState struct {
	C            <-chan networkmanager.State
	config       Config
	log          logger.Logger
	bus          bus.Interface
	connAttempts uint32
	webget       func(ch chan<- bool)
	webgetC      chan bool
	currentState networkmanager.State
	lastSent     bool
	timer        *time.Timer
}

func (cs *connectedState) ConnectTimeout() time.Duration {
	var timeout config.ConfigTimeDuration
	timeouts := cs.config.ConnectTimeouts
	if cs.connAttempts < uint32(len(timeouts)) {
		timeout = timeouts[cs.connAttempts]
	} else if len(timeouts) > 0 {
		timeout = cs.config.ConnectTimeouts[len(timeouts)-1]
	}
	cs.connAttempts++
	return timeout.Duration
}

func (cs *connectedState) Start() networkmanager.State {
	var initial networkmanager.State
	for {
		time.Sleep(cs.ConnectTimeout())
		cs.log.Debugf("Starting DBus connection attempt %d\n", cs.connAttempts)
		conn, err := cs.bus.Connect(networkmanager.BusInfo, cs.log)
		if err != nil {
			cs.log.Debugf("DBus connection attempt %d failed.\n", cs.connAttempts)
			continue
		}
		nm := networkmanager.New(conn, cs.log)

		// Get the current state.
		initial = nm.GetState()
		if initial == networkmanager.Unknown {
			cs.log.Debugf("Failed to get state at attempt.")
			conn.Close()
			continue
		}

		// set up the watch
		ch, err := nm.WatchState()
		if err != nil {
			cs.log.Debugf("Failed to set up the watch: %s", err)
			conn.Close()
			continue
		}

		cs.C = ch
		cs.log.Debugf("worked at attempt %d. Resetting counter.\n", cs.connAttempts)
		return initial
	}
}

func (cs *connectedState) connectedStateStep() (bool, error) {
	stabilizingTimeout := cs.config.StabilizingTimeout.Duration
	recheckTimeout := cs.config.RecheckTimeout.Duration
	log := cs.log
	done := false
	for !done {
		select {
		case v, ok := <-cs.C:
			if !ok {
				// tear it all down and start over
				return false, errors.New("Got not-OK from StateChanged watch")
			}
			log.Debugf("State changed to %s. Assuming disconnect.", v)
			if cs.lastSent == true {
				log.Infof("Sending 'disconnected'.")
				cs.lastSent = false
				done = true
			}
			cs.timer.Stop()
			cs.webgetC = nil
			cs.timer.Reset(stabilizingTimeout)
			cs.currentState = v

		case <-cs.timer.C:
			cs.timer.Stop()
			if cs.currentState == networkmanager.ConnectedGlobal {
				log.Debugf("May be connected; checking...")
				cs.webgetC = make(chan bool)
				go cs.webget(cs.webgetC)
			}

		case connected := <-cs.webgetC:
			cs.timer.Reset(recheckTimeout)
			log.Debugf("Connection check says: %t", connected)
			cs.webgetC = nil
			if connected {
				if cs.lastSent == false {
					log.Infof("Sending 'connected'.")
					cs.lastSent = true
					done = true
				}
			}
		}
	}
	return cs.lastSent, nil

}

// notify initial state and changes to it. Sends "false" as soon as it detects
// trouble, "true" after checking actual connectivity.
func ConnectedState(busType bus.Interface, config Config, log logger.Logger, out chan<- bool) {
	wg := webchecker.New(config.ConnectivityCheckURL, config.ConnectivityCheckMD5, log)
	cs := &connectedState{
		config: config,
		log:    log,
		bus:    busType,
		webget: wg.Webcheck,
	}

start:
	log.Infof("Sending initial 'disconnected'.")
	out <- false
	cs.lastSent = false
	cs.currentState = cs.Start()
	cs.timer = time.NewTimer(cs.config.StabilizingTimeout.Duration)

	for {
		v, err := cs.connectedStateStep()
		if err != nil {
			// tear it all down and start over
			log.Errorf("%s", err)
			goto start
		}
		out <- v
	}
}
