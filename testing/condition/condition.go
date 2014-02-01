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

// Package testing/condition implements a strategy family for use in testing.
package condition

import (
	"fmt"
	"strings"
	"sync"
)

type Interface interface {
	OK() bool
	String() string
}

// Work is a simple boolean condition; either it works all the time
// (when true), or it fails all the time (when false).
func Work(wk bool) work {
	return work(wk)
}

type work bool

func (c work) OK() bool {
	if c {
		return true
	} else {
		return false
	}
}
func (c work) String() string {
	if c {
		return "Always Working."
	} else {
		return "Never Working."
	}
}

var _ Interface = work(false)

// Fail2Work fails for the first n times its OK() method is checked,
// and then mysteriously starts working.
func Fail2Work(left int32) *fail2Work {
	c := new(fail2Work)
	c.Left = left
	return c
}

type fail2Work struct {
	Left int32
	lock sync.RWMutex
}

func (c *fail2Work) OK() bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.Left > 0 {
		c.Left--
		return false
	} else {
		return true
	}
}

func (c *fail2Work) String() string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if c.Left > 0 {
		return fmt.Sprintf("Still Broken, %d to go.", c.Left)
	} else {
		return "Working."
	}
}

var _ Interface = &fail2Work{}

// Not builds a condition that negates the one passed in.
func Not(sub Interface) *not {
	return &not{sub}
}

type not struct{ sub Interface }

func (c *not) OK() bool       { return !c.sub.OK() }
func (c *not) String() string { return fmt.Sprintf("Not %s", c.sub) }

var _ Interface = &not{}

type _iter struct {
	cond      Interface
	remaining int
}

func (i _iter) String() string { return fmt.Sprintf("%d of %s", i.remaining, i.cond) }

type chain struct {
	subs []*_iter
	lock sync.RWMutex
}

func (c *chain) OK() bool {
	var sub *_iter
	c.lock.Lock()
	defer c.lock.Unlock()
	for _, sub = range c.subs {
		if sub.remaining > 0 {
			sub.remaining--
			return sub.cond.OK()
		}
	}
	return sub.cond.OK()
}

func (c *chain) String() string {
	ss := make([]string, len(c.subs))
	c.lock.RLock()
	defer c.lock.RUnlock()
	for i, sub := range c.subs {
		ss[i] = sub.String()
	}
	return strings.Join(ss, " Then: ")
}

var _ Interface = new(chain)

// Chain(n1, cond1, n2, cond2, ...) returns cond1.OK() the first n1
// times OK() is called, cond2.OK() the following n2 times, etc.
func Chain(args ...interface{}) *chain {
	iters := make([]*_iter, 0, len(args)/2)
	for len(args) > 1 {
		rem := args[0].(int)
		sub := args[1].(Interface)
		iters = append(iters, &_iter{sub, rem})
		args = args[2:]
	}

	return &chain{subs: iters}
}
