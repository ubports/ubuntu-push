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

func (lhl *legacyHelperLauncher) Launch(_, progname, f1, f2 string) (string, error) {
	cmd := exec.Command(progname, f1, f2)
	cmd.Stdin = nil
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Start()
	if err != nil {
		return "", err
	}
	proc := cmd.Process
	if proc == nil {
		panic("cmd.Process is nil after successful cmd.Start()??")
	}
	id := strconv.FormatInt((int64)(proc.Pid), 36)
	go func() {
		err = cmd.Wait()
		if err != nil {
			// Helper failed, log output
			lhl.log.Errorf("Legacy helper failed. Stdout: %s", stdout.String())
			lhl.log.Errorf("Legacy helper failed. Stderr: %s", stderr.String())
		}
		lhl.done(id)
	}()

	return id, nil
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
