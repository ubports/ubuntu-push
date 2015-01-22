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

// package legacy implements a HelperLauncher for “legacy” applications.
package legacy

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/logger"
)

type legacyHelperLauncher struct {
	log  logger.Logger
	done func(string)
}

func New(log logger.Logger) *legacyHelperLauncher {
	return &legacyHelperLauncher{log: log}
}

func (lhl *legacyHelperLauncher) InstallObserver(done func(string)) error {
	lhl.done = done
	return nil
}

var legacyHelperDir = "/usr/lib/ubuntu-push-client/legacy-helpers"

func (lhl *legacyHelperLauncher) HelperInfo(app *click.AppId) (string, string) {
	return "", filepath.Join(legacyHelperDir, app.Application)
}

func (*legacyHelperLauncher) RemoveObserver() error { return nil }

type msg struct {
	id  string
	err error
}

func (lhl *legacyHelperLauncher) Launch(appId, progname, f1, f2 string) (string, error) {
	comm := make(chan msg)

	go func() {
		cmd := exec.Command(progname, f1, f2)
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Start()
		if err != nil {
			comm <- msg{"", err}
			return
		}
		proc := cmd.Process
		if proc == nil {
			panic("cmd.Process is nil after successful cmd.Start()??")
		}
		id := strconv.FormatInt((int64)(proc.Pid), 36)
		comm <- msg{id, nil}
		p_err := cmd.Wait()
		if p_err != nil {
			// Helper failed or got killed, log output/errors
			lhl.log.Errorf("legacy helper failed: appId: %v, helper: %v, pid: %v, error: %v, stdout: %#v, stderr: %#v.",
				appId, progname, id, p_err, stdout.String(), stderr.String())
		}
		lhl.done(id)
	}()
	msg := <-comm
	return msg.id, msg.err
}

func (lhl *legacyHelperLauncher) Stop(_, id string) error {
	pid, err := strconv.ParseInt(id, 36, 0)
	if err != nil {
		return err
	}
	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return err
	}
	return proc.Kill()
}
