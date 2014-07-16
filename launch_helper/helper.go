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

type trivialHelperLauncher struct {
	log   logger.Logger
	chOut chan *HelperResult
	chIn  chan *HelperInput
}

// a trivial HelperLauncher that doesn't launch anything at all
func NewTrivialHelperLauncher(log logger.Logger) HelperLauncher {
	return &trivialHelperLauncher{log: log}
}

func (triv *trivialHelperLauncher) Start() chan *HelperResult {
	triv.chOut = make(chan *HelperResult)
	triv.chIn = make(chan *HelperInput, InputBufferSize)

	go func() {
		for i := range triv.chIn {
			res := &HelperResult{Input: i}
			err := json.Unmarshal(i.Payload, &res.HelperOutput)
			if err != nil {
				triv.log.Debugf("failed to parse HelperOutput from message, leaving it alone: %v", err)
				res.Message = i.Payload
				res.Notification = nil
			}
			triv.chOut <- res
		}
	}()

	return triv.chOut
}

func (triv *trivialHelperLauncher) Stop() {
	close(triv.chIn)
}

func (triv *trivialHelperLauncher) Run(input *HelperInput) {
	triv.chIn <- input
}
