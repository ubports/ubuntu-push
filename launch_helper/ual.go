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

package launch_helper

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"

	"launchpad.net/go-xdg/v0"

	"launchpad.net/ubuntu-push/launch_helper/cual"
	"launchpad.net/ubuntu-push/logger"
)

type ualHelperLauncher struct {
	log   logger.Logger
	chOut chan *HelperResult
	chIn  chan *HelperInput
	cual  cual.HelperState
}

var newHelperState = cual.New

// a HelperLauncher that calls out to ubuntu-app-launch
func NewHelperLauncher(log logger.Logger) HelperLauncher {
	return &ualHelperLauncher{log: log}
}

func (ual *ualHelperLauncher) Start() chan *HelperResult {
	ual.chOut = make(chan *HelperResult)
	ual.chIn = make(chan *HelperInput, InputBufferSize)
	ual.cual = newHelperState(ual.log)

	// xxx handle error
	ual.cual.InstallObserver()

	go func() {
		for i := range ual.chIn {
			if ual.handleOne(i) != nil {
				ual.failOne(i)
			}
		}
	}()

	return ual.chOut
}

func (ual *ualHelperLauncher) Stop() {
	close(ual.chIn)
	// xxx handle error
	ual.cual.RemoveObserver()
}

func (ual *ualHelperLauncher) Run(input *HelperInput) {
	ual.chIn <- input
}

func (ual *ualHelperLauncher) failOne(input *HelperInput) {
	ual.log.Errorf("unable to get helper output; putting payload into message")
	ual.chOut <- &HelperResult{HelperOutput: HelperOutput{Message: input.Payload, Notification: nil}, Input: input}
}

func (ual *ualHelperLauncher) cleanupTempFiles(f1, f2 string) {
	os.Remove(f1)
	os.Remove(f2)
}

func (ual *ualHelperLauncher) handleOne(input *HelperInput) error {
	f1, f2, err := ual.createTempFiles(input)
	if err != nil {
		ual.log.Errorf("unable to create tempfiles: %v", err)
		return err
	}
	return ual.cual.Launch(&cual.HelperArgs{
		App:            input.App,
		NotificationId: input.NotificationId,
		Payload:        input.Payload,
		FileIn:         f1,
		FileOut:        f2,
		OneDone:        ual.OneDone,
	})
}

func (ual *ualHelperLauncher) OneDone(args *cual.HelperArgs) {
	input := &HelperInput{
		App:            args.App,
		NotificationId: args.NotificationId,
		Payload:        args.Payload,
	}

	payload, err := ioutil.ReadFile(args.FileOut)
	if err != nil {
		ual.log.Errorf("unable to read output from helper: %v", err)
	} else {
		res := &HelperResult{Input: input}
		err = json.Unmarshal(payload, &res.HelperOutput)
		if err != nil {
			ual.log.Debugf("failed to parse HelperOutput from helper output: %v", err)
		} else {
			ual.chOut <- res
		}
	}
	if err != nil {
		ual.failOne(input)
	}
	ual.cleanupTempFiles(args.FileIn, args.FileOut)
}

func (ual *ualHelperLauncher) createTempFiles(input *HelperInput) (f1 string, f2 string, err error) {
	f1, err = getTempFilename(input.App.Package)
	if err != nil {
		ual.log.Errorf("failed to create input file: %v", err)
		return
	}
	f2, err = getTempFilename(input.App.Package)
	if err == nil {
		err = ioutil.WriteFile(f1, input.Payload, os.ModeTemporary)
		if err == nil {
			return
		}
		ual.log.Errorf("failed to write to input file: %v", err)
		os.Remove(f2)
		f2 = ""
	} else {
		ual.log.Errorf("failed to create output file: %v", err)
	}
	os.Remove(f1)
	f1 = ""
	return
}

// helper helpers:

var xdgCacheHome = xdg.Cache.Home

func _getTempDir(pkgName string) (string, error) {
	tmpDir := path.Join(xdgCacheHome(), pkgName)
	err := os.MkdirAll(tmpDir, 0700)
	return tmpDir, err
}

var getTempDir = _getTempDir

func _getTempFilename(pkgName string) (string, error) {
	tmpDir, err := getTempDir(pkgName)
	if err != nil {
		return "", err
	}
	file, err := ioutil.TempFile(tmpDir, "push-helper")
	if err != nil {
		return "", err
	}
	defer file.Close()
	return file.Name(), nil
}

var getTempFilename = _getTempFilename
