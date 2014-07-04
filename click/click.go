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

// Package click exposes some utilities related to click packages
package click

import (
	"errors"
	"regexp"
)

type AppId struct {
	Package     string
	Application string
	Version     string
}

// from https://wiki.ubuntu.com/AppStore/Interfaces/ApplicationId
// except the version is made optional
var rx = regexp.MustCompile(`^([a-z0-9][a-z0-9+.-]+)_([a-zA-Z0-9+.-]+)(?:_([0-9][a-zA-Z0-9.+:~-]*))?$`)

var (
	InvalidAppId = errors.New("Invalid App Id")
)

func ParseAppId(id string) (*AppId, error) {
	m := rx.FindStringSubmatch(id)
	if len(m) == 0 {
		return nil, InvalidAppId
	}
	return &AppId{Package: m[1], Application: m[2], Version: m[3]}, nil
}

func AppInPackage(app string, pkg string) bool {
	id, _ := ParseAppId(app)
	return id != nil && id.Package == pkg
}
