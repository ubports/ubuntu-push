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
	"os"
	"os/exec"
	"path/filepath"

	"launchpad.net/go-xdg/v0"

	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/logger"
)

type Sound struct {
	player   string
	log      logger.Logger
	dataDirs func() []string
	dataFind func(string) (string, error)
}

func New(log logger.Logger) *Sound {
	return &Sound{player: "paplay", log: log, dataDirs: xdg.Data.Dirs, dataFind: xdg.Data.Find}
}

func (snd *Sound) Tags(app *click.AppId) map[string][]string {
	return nil
}

func (snd *Sound) Present(app *click.AppId, nid string, notification *launch_helper.Notification) bool {
	if notification == nil {
		panic("please check notification is not nil before calling present")
	}

	if notification.Sound == "" {
		snd.log.Debugf("[%s] notification has no Sound: %#v", nid, notification.Sound)
		return false
	}
	absPath := snd.findSoundFile(app, nid, notification.Sound)
	if absPath == "" {
		snd.log.Debugf("[%s] unable to find sound %s", nid, notification.Sound)
		return false
	}
	snd.log.Debugf("[%s] playing sound %s using %s", nid, absPath, snd.player)
	cmd := exec.Command(snd.player, absPath)
	err := cmd.Start()
	if err != nil {
		snd.log.Debugf("[%s] unable to play: %v", nid, err)
		return false
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			snd.log.Debugf("[%s] error playing sound %s: %v", nid, absPath, err)
		}
	}()
	return true
}

func (snd *Sound) findSoundFile(app *click.AppId, nid string, sound string) string {
	// XXX also support legacy appIds?
	// first, check package-specific
	absPath, err := snd.dataFind(filepath.Join(app.Package, sound))
	if err == nil {
		// ffffound
		return absPath
	}
	// next, check the XDG data dirs (but skip the first one -- that's "home")
	// XXX should we only check in $XDG/sounds ? (that's for sound *themes*...)
	for _, dir := range snd.dataDirs()[1:] {
		absPath := filepath.Join(dir, sound)
		_, err := os.Stat(absPath)
		if err == nil {
			return absPath
		}
	}
	return ""
}
