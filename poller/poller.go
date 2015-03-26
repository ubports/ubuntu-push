/*
 Copyright 2014 Canonical Ltd.

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
	times        Times
	log          logger.Logger
	powerd       powerd.Powerd
	polld        polld.Polld
	cookie       string
	sessionState stater
}

func New(setup *PollerSetup) Poller {
	return &poller{
		times:        setup.Times,
		log:          setup.Log,
		powerd:       nil,
		polld:        nil,
		sessionState: setup.SessionStateGetter,
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
	go p.run(wakeupCh, doneCh)
	return nil
}

func (p *poller) run(wakeupCh <-chan bool, doneCh <-chan bool) {
	var lockCookie string

	for {
		lockCookie = p.step(wakeupCh, doneCh, lockCookie)
	}
}

func (p *poller) step(wakeupCh <-chan bool, doneCh <-chan bool, lockCookie string) string {

	t := time.Now().Add(p.times.AlarmInterval).Truncate(time.Second)
	_, err := p.powerd.RequestWakeup("ubuntu push client", t)
	if err != nil {
		p.log.Errorf("RequestWakeup got %v", err)
		// Don't do this too quickly. Pretend we are just skipping one wakeup
		time.Sleep(p.times.AlarmInterval)
		return lockCookie
	}
	p.log.Debugf("requested wakeup at %s", t)
	if lockCookie != "" {
		if err := p.powerd.ClearWakelock(lockCookie); err != nil {
			p.log.Errorf("ClearWakelock(%#v) got %v", lockCookie, err)
		} else {
			p.log.Debugf("cleared wakelock cookie %s.", lockCookie)
		}
		lockCookie = ""
	}
	for b := range wakeupCh {
		if !b {
			panic("WatchWakeups channel produced a false value (??)")
		}
		// the channel will produce a true for every
		// wakeup, not only the one we asked for
		now := time.Now()
		p.log.Debugf("got woken up; time is %s (𝛥: %s)", now, now.Sub(t))
		if !now.Before(t) {
			break
		}
	}
	lockCookie, err = p.powerd.RequestWakelock("ubuntu push client")
	if err != nil {
		p.log.Errorf("RequestWakelock got %v", err)
		return lockCookie
	}
	p.log.Debugf("got wakelock cookie of %s, checking conn state", lockCookie)
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
