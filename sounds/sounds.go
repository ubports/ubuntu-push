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

package sounds

import (
	"os/exec"

	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/logger"
)

type Sound struct {
	player string
	log    logger.Logger
}

func New(log logger.Logger) *Sound {
	return &Sound{player: "paplay", log: log}
}

func (snd *Sound) Present(_, _ string, notification *launch_helper.Notification) bool {
	if notification == nil || notification.Sound == "" {
		snd.log.Debugf("no notification or no Sound in the notification; doing nothing: %#v", notification)
		return false
	}
	snd.log.Debugf("playing sound %s using %s", notification.Sound, snd.player)
	cmd := exec.Command(snd.player, notification.Sound)
	err := cmd.Start()
	if err != nil {
		snd.log.Debugf("unable to play: %v", err)
		return false
	}
	return true
}
