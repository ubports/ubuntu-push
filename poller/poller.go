/*
 Copyright 2014-2015 Canonical Ltd.

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

// Package poller implements Poller, a thing that uses (hw) alarms to
// wake the device up from deep sleep periodically, check for
// notifications, and poke polld.
package poller

import (
	"errors"
	"sync"
	"time"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/bus/networkmanager"
	"launchpad.net/ubuntu-push/bus/polld"
	"launchpad.net/ubuntu-push/bus/powerd"
	"launchpad.net/ubuntu-push/bus/urfkill"
	"launchpad.net/ubuntu-push/client/session"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/util"
)

var (
	ErrUnconfigured   = errors.New("not configured")
	ErrAlreadyStarted = errors.New("already started")
	ErrNotStarted     = errors.New("not started")
)

type stater interface {
	State() session.ClientSessionState
}

type Times struct {
	AlarmInterval      time.Duration
	SessionStateSettle time.Duration
	NetworkWait        time.Duration
	PolldWait          time.Duration
	DoneWait           time.Duration
}

type Poller interface {
	IsConnected() bool
	Start() error
	Run() error
}

type PollerSetup struct {
	Times              Times
	Log                logger.Logger
	SessionStateGetter stater
}

type poller struct {
	times                Times
	log                  logger.Logger
	nm                   networkmanager.NetworkManager
	powerd               powerd.Powerd
	polld                polld.Polld
	urfkill              urfkill.URfkill
	cookie               string
	sessionState         stater
	requestWakeupCh      chan struct{}
	requestedWakeupErrCh chan error
	holdsWakeLockCh      chan bool
}

func New(setup *PollerSetup) Poller {
	return &poller{
		times:                setup.Times,
		log:                  setup.Log,
		powerd:               nil,
		polld:                nil,
		sessionState:         setup.SessionStateGetter,
		requestWakeupCh:      make(chan struct{}),
		requestedWakeupErrCh: make(chan error),
		holdsWakeLockCh:      make(chan bool),
	}
}

func (p *poller) IsConnected() bool {
	return p.sessionState.State() == session.Running
}

func (p *poller) Start() error {
	if p.log == nil {
		return ErrUnconfigured
	}
	if p.powerd != nil || p.polld != nil {
		return ErrAlreadyStarted
	}
	nmEndp := bus.SystemBus.Endpoint(networkmanager.BusAddress, p.log)
	powerdEndp := bus.SystemBus.Endpoint(powerd.BusAddress, p.log)
	polldEndp := bus.SessionBus.Endpoint(polld.BusAddress, p.log)
	urEndp := bus.SystemBus.Endpoint(urfkill.BusAddress, p.log)
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		n := util.NewAutoRedialer(nmEndp).Redial()
		p.log.Debugf("NetworkManager dialed on try %d", n)
		wg.Done()
	}()
	go func() {
		n := util.NewAutoRedialer(powerdEndp).Redial()
		p.log.Debugf("powerd dialed on try %d", n)
		wg.Done()
	}()
	go func() {
		n := util.NewAutoRedialer(polldEndp).Redial()
		p.log.Debugf("polld dialed in on try %d", n)
		wg.Done()
	}()
	go func() {
		n := util.NewAutoRedialer(urEndp).Redial()
		p.log.Debugf("URfkill dialed on try %d", n)
		wg.Done()
	}()
	wg.Wait()

	p.nm = networkmanager.New(nmEndp, p.log)
	p.powerd = powerd.New(powerdEndp, p.log)
	p.polld = polld.New(polldEndp, p.log)
	p.urfkill = urfkill.New(urEndp, p.log)

	return nil
}

func (p *poller) Run() error {
	if p.log == nil {
		return ErrUnconfigured
	}
	if p.nm == nil || p.powerd == nil || p.polld == nil || p.urfkill == nil {
		return ErrNotStarted
	}
	wakeupCh, err := p.powerd.WatchWakeups()
	if err != nil {
		return err
	}
	doneCh, err := p.polld.WatchDones()
	if err != nil {
		return err
	}
	flightMode := p.urfkill.IsFlightMode()
	wirelessEnabled := p.nm.GetWirelessEnabled()
	flightModeCh, _, err := p.urfkill.WatchFlightMode()
	if err != nil {
		return err
	}
	wirelessEnabledCh, _, err := p.nm.WatchWirelessEnabled()
	if err != nil {
		return err
	}

	filteredWakeUpCh := make(chan bool)
	go p.control(wakeupCh, filteredWakeUpCh, flightMode, flightModeCh, wirelessEnabled, wirelessEnabledCh)
	go p.run(filteredWakeUpCh, doneCh)
	return nil
}

func (p *poller) doRequestWakeup(delta time.Duration) (time.Time, string, error) {
	t := time.Now().Add(delta).Truncate(time.Second)
	cookie, err := p.powerd.RequestWakeup("ubuntu push client", t)
	if err == nil {
		p.log.Debugf("requested wakeup at %s", t)
	} else {
		p.log.Errorf("RequestWakeup got %v", err)
		t = time.Time{}
		cookie = ""
	}
	return t, cookie, err
}

func (p *poller) control(wakeupCh <-chan bool, filteredWakeUpCh chan<- bool, flightMode bool, flightModeCh <-chan bool, wirelessEnabled bool, wirelessEnabledCh <-chan bool) {
	dontPoll := flightMode && !wirelessEnabled
	var t time.Time
	cookie := ""
	holdsWakeLock := false
	for {
		select {
		case holdsWakeLock = <-p.holdsWakeLockCh:
		case <-p.requestWakeupCh:
			if !t.IsZero() || dontPoll {
				// earlier wakeup or we shouldn't be polling
				// => don't request wakeup
				if dontPoll {
					p.log.Debugf("skip requesting wakeup")
				}
				p.requestedWakeupErrCh <- nil
				break
			}
			var err error
			t, cookie, err = p.doRequestWakeup(p.times.AlarmInterval)
			p.requestedWakeupErrCh <- err
		case b := <-wakeupCh:
			// seems we get here also on clear wakeup, oh well
			if !b {
				panic("WatchWakeups channel produced a false value (??)")
			}
			// the channel will produce a true for every
			// wakeup, not only the one we asked for
			now := time.Now()
			if t.IsZero() {
				p.log.Debugf("got woken up; time is %s", now)
			} else {
				p.log.Debugf("got woken up; time is %s (𝛥: %s)", now, now.Sub(t))
				if !now.Before(t) {
					t = time.Time{}
					filteredWakeUpCh <- true
				}
			}
		case flightMode = <-flightModeCh:
		case wirelessEnabled = <-wirelessEnabledCh:
		}
		newDontPoll := flightMode && !wirelessEnabled
		p.log.Debugf("control: flightMode:%v wirelessEnabled:%v prevDontPoll:%v dontPoll:%v wakeupReq:%v holdsWakeLock:%v", flightMode, wirelessEnabled, dontPoll, newDontPoll, !t.IsZero(), holdsWakeLock)
		if newDontPoll != dontPoll {
			if dontPoll = newDontPoll; dontPoll {
				if !t.IsZero() {
					err := p.powerd.ClearWakeup(cookie)
					if err == nil {
						// cleared
						t = time.Time{}
						p.log.Debugf("cleared wakeup")
					} else {
						p.log.Errorf("ClearWakeup got %v", err)
					}
				}
			} else {
				if t.IsZero() && !holdsWakeLock {
					// reschedule soon
					t, cookie, _ = p.doRequestWakeup(p.times.NetworkWait / 20)
				}
			}
		}
	}
}

func (p *poller) requestWakeup() error {
	p.requestWakeupCh <- struct{}{}
	return <-p.requestedWakeupErrCh
}

func (p *poller) holdsWakeLock(has bool) {
	p.holdsWakeLockCh <- has
}

func (p *poller) run(wakeupCh <-chan bool, doneCh <-chan bool) {
	var lockCookie string

	for {
		lockCookie = p.step(wakeupCh, doneCh, lockCookie)
	}
}

func (p *poller) step(wakeupCh <-chan bool, doneCh <-chan bool, lockCookie string) string {

	err := p.requestWakeup()
	if err != nil {
		// Don't do this too quickly. Pretend we are just skipping one wakeup
		time.Sleep(p.times.AlarmInterval)
		return lockCookie
	}
	p.holdsWakeLock(false)
	if lockCookie != "" {
		if err := p.powerd.ClearWakelock(lockCookie); err != nil {
			p.log.Errorf("ClearWakelock(%#v) got %v", lockCookie, err)
		} else {
			p.log.Debugf("cleared wakelock cookie %s.", lockCookie)
		}
		lockCookie = ""
	}
	<-wakeupCh
	lockCookie, err = p.powerd.RequestWakelock("ubuntu push client")
	if err != nil {
		p.log.Errorf("RequestWakelock got %v", err)
		return lockCookie
	}
	p.holdsWakeLock(true)
	p.log.Debugf("got wakelock cookie of %s, checking conn state", lockCookie)
	// XXX killed as part of bug #1435109 troubleshooting, remove cfg if remains unused
	// time.Sleep(p.times.SessionStateSettle)
	for i := 0; i < 20; i++ {
		if p.IsConnected() {
			p.log.Debugf("iter %02d: connected", i)
			break
		}
		p.log.Debugf("iter %02d: not connected, sleeping for %s", i, p.times.NetworkWait/20)
		time.Sleep(p.times.NetworkWait / 20)
		p.log.Debugf("iter %02d: slept", i)
	}
	if !p.IsConnected() {
		p.log.Errorf("not connected after %s; giving up", p.times.NetworkWait)
	} else {
		p.log.Debugf("poking polld.")
		// drain the doneCH
	drain:
		for {
			select {
			case <-doneCh:
			default:
				break drain
			}
		}

		if err := p.polld.Poll(); err != nil {
			p.log.Errorf("Poll got %v", err)
		} else {
			p.log.Debugf("waiting for polld to signal Done.")
			select {
			case b := <-doneCh:
				if !b {
					panic("WatchDones channel produced a false value (??)")
				}
				p.log.Debugf("polld Done.")
			case <-time.After(p.times.PolldWait):
				p.log.Errorf("polld still not done after %s; giving up", p.times.PolldWait)
			}
		}

		// XXX check whether something was actually done before waiting
		p.log.Debugf("sleeping for DoneWait %s", p.times.DoneWait)
		time.Sleep(p.times.DoneWait)
		p.log.Debugf("slept")
	}

	return lockCookie
}
