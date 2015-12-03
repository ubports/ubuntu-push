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
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"launchpad.net/go-xdg/v0"

	"launchpad.net/ubuntu-push/bus/accounts"
	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/logger"
)

type Sound struct {
	player   string
	log      logger.Logger
	acc      accounts.Accounts
	fallback string
	dataDirs func() []string
	dataFind func(string) (string, error)
}

func New(log logger.Logger, acc accounts.Accounts, fallback string) *Sound {
	return &Sound{
		player:   "paplay",
		log:      log,
		acc:      acc,
		fallback: fallback,
		dataDirs: xdg.Data.Dirs,
		dataFind: xdg.Data.Find,
	}
}

func (snd *Sound) Present(app *click.AppId, nid string, notification *launch_helper.Notification) bool {
	if notification == nil {
		panic("please check notification is not nil before calling present")
	}

	absPath := snd.GetSound(app, nid, notification)
	if absPath == "" {
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

// Returns the absolute path of the sound to be played for app, nid and notification.
func (snd *Sound) GetSound(app *click.AppId, nid string, notification *launch_helper.Notification) string {

	if snd.acc.SilentMode() {
		snd.log.Debugf("[%s] no sounds: silent mode on.", nid)
		return ""
	}

	fallback := snd.acc.MessageSoundFile()
	if fallback == "" {
		fallback = snd.fallback
	}

	sound := notification.Sound(fallback)
	if sound == "" {
		snd.log.Debugf("[%s] notification has no Sound: %#v", nid, sound)
		return ""
	}
	absPath := snd.findSoundFile(app, nid, sound)
	if absPath == "" {
		snd.log.Debugf("[%s] unable to find sound %s", nid, sound)
	}
	return absPath
}

// Removes all cruft from path, ensures it's a "forward" path.
func (snd *Sound) cleanPath(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if strings.Contains(cleaned, "../") {
		return "", errors.New("Path escaping xdg attempt")
	}
	return cleaned, nil
}

func (snd *Sound) findSoundFile(app *click.AppId, nid string, sound string) string {
	// XXX also support legacy appIds?
	// first, check package-specific
	sound, err := snd.cleanPath(sound)
	if err != nil {
		// bad boy
		return ""
	}
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
