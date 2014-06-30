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

// launch_helper wraps ubuntu_app_launch to enable using application
// helpers. The useful part is HelperRunner
package launch_helper

import (
	"encoding/json"

	"launchpad.net/ubuntu-push/logger"
)

type HelperLauncher interface {
	Run(appId string, message []byte) *HelperOutput
}

type trivialHelperLauncher struct {
	log logger.Logger
}

// a trivial HelperLauncher that doesn't launch anything at all
func NewTrivialHelperLauncher(log logger.Logger) HelperLauncher {
	return &trivialHelperLauncher{log}
}

func (triv *trivialHelperLauncher) Run(appId string, message []byte) *HelperOutput {
	out := new(HelperOutput)
	err := json.Unmarshal(message, out)
	if err == nil {
		return out
	}
	triv.log.Debugf("failed to parse HelperOutput from message, leaving it alone: %v", err)
	out.Message = message
	out.Notification = nil

	return out
}
