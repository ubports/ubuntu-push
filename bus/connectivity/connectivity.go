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

// Package connectivity implements a single, simple stream of booleans
// to answer the question "are we connected?".
//
// It can potentially fire two falses in a row, if a disconnected
// state is followed by a dbus watch error. Other than that, it's edge
// triggered.
package connectivity

import (
	"errors"
	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/bus/networkmanager"
	"launchpad.net/ubuntu-push/config"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/util"
	"time"
)

// The configuration for ConnectedState, intended to be populated from a config file.
type ConnectivityConfig struct {
	// how long to wait after a state change to make sure it's "stable"
	// before acting on it
	StabilizingTimeout config.ConfigTimeDuration `json:"stabilizing_timeout"`
	// How long to wait between online connectivity checks.
	RecheckTimeout config.ConfigTimeDuration `json:"recheck_timeout"`
	// The URL against which to do the connectivity check.
	ConnectivityCheckURL string `json:"connectivity_check_url"`
	// The expected MD5 of the content at the ConnectivityCheckURL
	ConnectivityCheckMD5 string `json:"connectivity_check_md5"`
}

type connectedState struct {
	networkStateCh <-chan networkmanager.State
	networkConCh   <-chan string
	config         ConnectivityConfig
	log            logger.Logger
	endp           bus.Endpoint
	connAttempts   uint32
	webget         func(ch chan<- bool)
	webgetCh       chan bool
	currentState   networkmanager.State
	lastSent       bool
	timer          *time.Timer
}

// start connects to the bus, gets the initial NetworkManager state, and sets
// up the watch.
func (cs *connectedState) start() networkmanager.State {
	var initial networkmanager.State
	var stateCh <-chan networkmanager.State
	var primary string
	var conCh <-chan string
	var err error
	for {
		ar := util.NewAutoRedialer(cs.endp)
		cs.connAttempts += ar.Redial()
		nm := networkmanager.New(cs.endp, cs.log)

		// Get the current state.
		initial = nm.GetState()
		if initial == networkmanager.Unknown {
			cs.log.Debugf("Failed to get state.")
			goto Continue
		}

		// set up the watch
		stateCh, err = nm.WatchState()
		if err != nil {
			cs.log.Debugf("failed to set up the state watch: %s", err)
			goto Continue
		}

		primary = nm.GetPrimaryConnection()
		cs.log.Debugf("primary connection starts as %#v", primary)

		conCh, err = nm.WatchPrimaryConnection()
		if err != nil {
			cs.log.Debugf("failed to set up the connection watch: %s", err)
			goto Continue
		}

		cs.networkStateCh = stateCh
		cs.networkConCh = conCh

		return initial

	Continue:
		cs.endp.Close()
		time.Sleep(10 * time.Millisecond) // that should cool things
	}
}

// connectedStateStep takes one step forwards in the “am I connected?”
// answering state machine.
func (cs *connectedState) connectedStateStep() (bool, error) {
	stabilizingTimeout := cs.config.StabilizingTimeout.Duration
	recheckTimeout := cs.config.RecheckTimeout.Duration
	log := cs.log

Loop:
	for {
		select {
		case <-cs.networkConCh:
			cs.webgetCh = nil
			cs.timer.Reset(stabilizingTimeout)
			log.Debugf("PrimaryConnection changed. Assuming disconnect.")
			if cs.lastSent == true {
				cs.lastSent = false
				break Loop
			}

		case v, ok := <-cs.networkStateCh:
			if !ok {
				// tear it all down and start over
				return false, errors.New("Got not-OK from StateChanged watch")
			}
			cs.webgetCh = nil
			cs.currentState = v
			cs.timer.Reset(stabilizingTimeout)
			log.Debugf("State changed to %s. Assuming disconnect.", v)
			if cs.lastSent == true {
				log.Infof("Sending 'disconnected'.")
				cs.lastSent = false
				break Loop
			}

		case <-cs.timer.C:
			if cs.currentState == networkmanager.ConnectedGlobal {
				log.Debugf("May be connected; checking...")
				cs.webgetCh = make(chan bool)
				go cs.webget(cs.webgetCh)
			}

		case connected := <-cs.webgetCh:
			cs.timer.Reset(recheckTimeout)
			log.Debugf("Connection check says: %t", connected)
			cs.webgetCh = nil
			if connected && cs.lastSent == false {
				log.Infof("Sending 'connected'.")
				cs.lastSent = true
				break Loop
			}
		}
	}
	return cs.lastSent, nil
}

// ConnectedState sends the initial NetworkManager state and changes to it
// over the "out" channel. Sends "false" as soon as it detects trouble, "true"
// after checking actual connectivity.
//
// The endpoint need not be dialed; connectivity will Dial() and Close()
// it as it sees fit.
func ConnectedState(endp bus.Endpoint, config ConnectivityConfig, log logger.Logger, out chan<- bool) {
	wg := NewWebchecker(config.ConnectivityCheckURL, config.ConnectivityCheckMD5, 10*time.Second, log)
	cs := &connectedState{
		config: config,
		log:    log,
		endp:   endp,
		webget: wg.Webcheck,
	}

Start:
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
			goto Start
		}
		out <- v
	}
}
