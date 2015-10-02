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
	"launchpad.net/ubuntu-push/bus/polld"
	"launchpad.net/ubuntu-push/bus/powerd"
	"launchpad.net/ubuntu-push/client/session"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/util"
)

var (
	ErrUnconfigured   = errors.New("not configured")
	ErrAlreadyStarted = errors.New("already started")
	ErrNotStarted     = errors.New("not started")
)

// type PrematureWakeupError struct {
//     msg string // description of error
// }

type stater interface {
	State() session.ClientSessionState
}

type Times struct {
	AlarmInterval      time.Duration
	SessionStateSettle time.Duration
	NetworkWait        time.Duration
	PolldWait          time.Duration
	DoneWait           time.Duration
	BusyWait           time.Duration
}

type Poller interface {
	IsConnected() bool
	Start() error
	Run() error
	HasConnectivity(bool)
}

type PollerSetup struct {
	Times              Times
	Log                logger.Logger
	SessionStateGetter stater
}

type poller struct {
	times                Times
	log                  logger.Logger
	powerd               powerd.Powerd
	polld                polld.Polld
	cookie               string
	sessionState         stater
	connCh               chan bool
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
		connCh:               make(chan bool, 1),
		requestWakeupCh:      make(chan struct{}),
		requestedWakeupErrCh: make(chan error),
		holdsWakeLockCh:      make(chan bool),
	}
}

func (p *poller) IsConnected() bool {
	return p.sessionState.State() == session.Running
}

func (p *poller) HasConnectivity(hasConn bool) {
	p.connCh <- hasConn
}

func (p *poller) Start() error {
	if p.log == nil {
		return ErrUnconfigured
	}
	if p.powerd != nil || p.polld != nil {
		return ErrAlreadyStarted
	}

	powerdEndp := bus.SystemBus.Endpoint(powerd.BusAddress, p.log)
	polldEndp := bus.SessionBus.Endpoint(polld.BusAddress, p.log)

	var wg sync.WaitGroup
	wg.Add(2)
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
	wg.Wait()

	p.powerd = powerd.New(powerdEndp, p.log)
	p.polld = polld.New(polldEndp, p.log)

	// busy sleep loop to workaround go's timer/sleep
	// not accounting for time when the system is suspended
	// see https://bugs.launchpad.net/ubuntu/+source/ubuntu-push/+bug/1435109
	if p.times.BusyWait > 0 {
		p.log.Debugf("starting busy loop with %s interval", p.times.BusyWait)
		go func() {
			for {
				time.Sleep(p.times.BusyWait)
			}
		}()
	} else {
		p.log.Debugf("skipping busy loop")
	}

	return nil
}

func (p *poller) Run() error {
	if p.log == nil {
		return ErrUnconfigured
	}
	if p.powerd == nil || p.polld == nil {
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
	filteredWakeUpCh := make(chan bool)
	go p.control(wakeupCh, filteredWakeUpCh)
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

func (p *poller) control(wakeupCh <-chan bool, filteredWakeUpCh chan<- bool) {
	// Assume a connection, and poll immediately.
	connected := true
	dontPoll := !connected
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
				if (!t.IsZero()) {
					p.log.Debugf("skip requesting wakeup due to IsZero")
				}
				if dontPoll {
					p.log.Debugf("skip requesting wakeup due to dontPoll")
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
		case state := <-p.connCh:
			connected = state
			p.log.Debugf("control: connected:%v", state)
		}
		newDontPoll := !connected
		p.log.Debugf("control: prevDontPoll:%v dontPoll:%v wakeupReq:%v holdsWakeLock:%v", dontPoll, newDontPoll, !t.IsZero(), holdsWakeLock)
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
					var err error
					t, cookie, err = p.doRequestWakeup(p.times.NetworkWait / 20)
						p.log.Debugf("reschedule")
					if err != nil {
						p.requestedWakeupErrCh <- err
					}
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

	p.log.Debugf("step: called")
	err := p.requestWakeup()
	if err != nil {
		// Don't do this too quickly. Pretend we are just skipping one wakeup
		p.log.Debugf("step: p.requestWakeup() ERROR:%v", err)
		time.Sleep(p.times.AlarmInterval)
		return lockCookie
	} else {
		p.log.Debugf("step: p.requestWakeup() OK")
	}
	p.log.Debugf("step: p.holdsWakeLock(false)")
	p.holdsWakeLock(false)
	if lockCookie != "" {
		if err := p.powerd.ClearWakelock(lockCookie); err != nil {
			p.log.Errorf("ClearWakelock(%#v) got %v", lockCookie, err)
		} else {
			p.log.Debugf("cleared wakelock cookie %s.", lockCookie)
		}
		lockCookie = ""
	}
	p.log.Debugf("step: before wakeupCh")
	select {
	case <-wakeupCh:
	case <-p.requestedWakeupErrCh:
		break
	}
	//<-wakeupCh
	p.log.Debugf("step: after wakeupCh")
	lockCookie, err = p.powerd.RequestWakelock("ubuntu push client")
	if err != nil {
		p.log.Errorf("RequestWakelock got %v", err)
		return lockCookie
	}
	p.holdsWakeLock(true)
	p.log.Debugf("got wakelock cookie of %s, checking conn state", lockCookie)
	time.Sleep(p.times.SessionStateSettle)
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
