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
	"launchpad.net/ubuntu-push/connectivity/webchecker"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/networkmanager"
	"time"
)

// the configuration for ConnectedState, with the idea that you'd populate it
// from a config file.
type Config struct {
	// a list of timeouts, for backoff. Should be roughly doubling.
	ConnectTimeouts []config.ConfigTimeDuration
	// how long to wait after a state change to make sure it's "stable"
	// before acting on it
	StabilizingTimeout config.ConfigTimeDuration
	// How long to wait between online connectivity checks.
	RecheckTimeout config.ConfigTimeDuration
	// The URL against which to do the connectivity check.
	ConnectivityCheckURL string
	// The expected MD5 of the content at the ConnectivityCheckURL
	ConnectivityCheckMD5 string
}

type connectedState struct {
	networkStateCh <-chan networkmanager.State
	config         Config
	log            logger.Logger
	bus            bus.Bus
	connAttempts   uint32
	webget         func(ch chan<- bool)
	webgetC        chan bool
	currentState   networkmanager.State
	lastSent       bool
	timer          *time.Timer
}

// implements the logic for connect timeouts backoff
//
// (walk the list of timeouts, and repeat the last one until done; cope with
// the list being empty; keep track of connection attempts).
func (cs *connectedState) connectTimeout() time.Duration {
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

// start connects to the bus, gets the initial NetworkManager state, and sets
// up the watch.
func (cs *connectedState) start() networkmanager.State {
	var initial networkmanager.State
	for {
		time.Sleep(cs.connectTimeout())
		cs.log.Debugf("Starting DBus connection attempt %d\n", cs.connAttempts)
		conn, err := cs.bus.Connect(networkmanager.BusAddress, cs.log)
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

		cs.networkStateCh = ch
		cs.log.Debugf("worked at attempt %d. Resetting counter.\n", cs.connAttempts)
		return initial
	}
}

// connectedStateStep takes one step forwards in the “am I connected?”
// answering state machine.
func (cs *connectedState) connectedStateStep() (bool, error) {
	stabilizingTimeout := cs.config.StabilizingTimeout.Duration
	recheckTimeout := cs.config.RecheckTimeout.Duration
	log := cs.log
loop:
	for {
		select {
		case v, ok := <-cs.networkStateCh:
			if !ok {
				// tear it all down and start over
				return false, errors.New("Got not-OK from StateChanged watch")
			}
			cs.webgetC = nil
			cs.currentState = v
			cs.timer.Reset(stabilizingTimeout)
			log.Debugf("State changed to %s. Assuming disconnect.", v)
			if cs.lastSent == true {
				log.Infof("Sending 'disconnected'.")
				cs.lastSent = false
				break loop
			}

		case <-cs.timer.C:
			if cs.currentState == networkmanager.ConnectedGlobal {
				log.Debugf("May be connected; checking...")
				cs.webgetC = make(chan bool)
				go cs.webget(cs.webgetC)
			}

		case connected := <-cs.webgetC:
			cs.timer.Reset(recheckTimeout)
			log.Debugf("Connection check says: %t", connected)
			cs.webgetC = nil
			if connected && cs.lastSent == false {
				log.Infof("Sending 'connected'.")
				cs.lastSent = true
				break loop
			}
		}
	}
	return cs.lastSent, nil
}

// ConnectedState sends the initial NetworkManager state and changes to it
// over the "out" channel. Sends "false" as soon as it detects trouble, "true"
// after checking actual connectivity.
func ConnectedState(busType bus.Bus, config Config, log logger.Logger, out chan<- bool) {
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
	cs.currentState = cs.start()
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
