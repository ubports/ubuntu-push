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

// Package click exposes some utilities related to click packages and
// wraps libclick to check if packages are installed.
package click

import (
	"errors"
	"regexp"
	"sync"

	"launchpad.net/ubuntu-push/click/cclick"
)

// AppId holds a parsed application id.
type AppId struct {
	Package     string
	Application string
	Version     string
}

// from https://wiki.ubuntu.com/AppStore/Interfaces/ApplicationId
// except the version is made optional
var rx = regexp.MustCompile(`^([a-z0-9][a-z0-9+.-]+)_([a-zA-Z0-9+.-]+)(?:_([0-9][a-zA-Z0-9.+:~-]*))?$`)

var (
	ErrInvalidAppId = errors.New("invalid application id")
)

func ParseAppId(id string) (*AppId, error) {
	m := rx.FindStringSubmatch(id)
	if len(m) == 0 {
		return nil, ErrInvalidAppId
	}
	return &AppId{Package: m[1], Application: m[2], Version: m[3]}, nil
}

func AppInPackage(appId, pkgname string) bool {
	id, _ := ParseAppId(appId)
	return id != nil && id.Package == pkgname
}

// ClickUser exposes the click package registry for the user.
type ClickUser struct {
	ccu  cclick.CClickUser
	lock sync.Mutex
}

// User makes a new ClickUser object for the current user.
func User() (*ClickUser, error) {
	cu := new(ClickUser)
	err := cu.ccu.CInit(cu)
	if err != nil {
		return nil, err
	}
	return cu, nil
}

// HasPackage checks if the appId is installed for user.
func (cu *ClickUser) HasPackage(appId string) bool {
	cu.lock.Lock()
	defer cu.lock.Unlock()
	id, err := ParseAppId(appId)
	if err != nil {
		return false
	}
	if id.Version != "" {
		return cu.ccu.CGetVersion(id.Package) == id.Version
	} else {
		return cu.ccu.CHasPackageName(id.Package)
	}
}
