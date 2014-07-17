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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"

	"launchpad.net/go-xdg/v0"

	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/launch_helper/cual"
	"launchpad.net/ubuntu-push/logger"
)

var (
	ErrCantFindHelper = errors.New("can't find helper")
)

type HelperArgs struct {
	Input      *HelperInput
	AppId      string
	FileIn     string
	FileOut    string
	Id         string
	Timer      *time.Timer
	ForcedStop bool
}

type ualHelperLauncher struct {
	log        logger.Logger
	chOut      chan *HelperResult
	chIn       chan *HelperInput
	cual       cual.HelperState
	lock       sync.Mutex
	hmap       map[string]*HelperArgs
	maxRuntime time.Duration
}

var newHelperState = cual.New

// a HelperLauncher that calls out to ubuntu-app-launch
func NewHelperLauncher(log logger.Logger) HelperLauncher {
	return &ualHelperLauncher{
		log:        log,
		hmap:       make(map[string]*HelperArgs),
		maxRuntime: 5 * time.Second,
	}
}

func (ual *ualHelperLauncher) Start() chan *HelperResult {
	ual.chOut = make(chan *HelperResult)
	ual.chIn = make(chan *HelperInput, InputBufferSize)
	ual.cual = newHelperState(ual.log, ual)

	err := ual.cual.InstallObserver()
	if err != nil {
		panic(fmt.Errorf("failed to install helper observer: %v", err))
	}

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
	err := ual.cual.RemoveObserver()
	if err != nil {
		panic(fmt.Errorf("failed to remove helper observer: %v", err))
	}
}

func (ual *ualHelperLauncher) Run(input *HelperInput) {
	ual.chIn <- input
}

func (ual *ualHelperLauncher) failOne(input *HelperInput) {
	ual.log.Errorf("unable to get helper output; putting payload into message")
	ual.chOut <- &HelperResult{HelperOutput: HelperOutput{Message: input.Payload, Notification: nil}, Input: input}
}

func (ual *ualHelperLauncher) cleanupTempFiles(f1, f2 string) {
	if f1 != "" {
		os.Remove(f1)
	}
	if f2 != "" {
		os.Remove(f2)
	}
}

func _helperInfo(app *click.AppId) (string, string) {
	return app.Helper()
}

var helperInfo = _helperInfo

func (ual *ualHelperLauncher) handleOne(input *HelperInput) error {
	helperAppId, helperExec := helperInfo(input.App)
	if helperAppId == "" || helperExec == "" {
		ual.log.Errorf("can't locate helper for app")
		return ErrCantFindHelper
	}
	ual.log.Debugf("using helper %s (exec: %s) for app %s", helperAppId, helperExec, input.App)
	var f1, f2 string
	f1, err := ual.createInputTempFile(input)
	defer func() {
		if err != nil {
			ual.cleanupTempFiles(f1, f2)
		}
	}()
	if err != nil {
		ual.log.Errorf("unable to create input tempfile: %v", err)
		return err
	}
	f2, err = ual.createOutputTempFile(input)
	if err != nil {
		ual.log.Errorf("unable to create output tempfile: %v", err)
		return err
	}

	args := HelperArgs{
		AppId:   helperAppId,
		Input:   input,
		FileIn:  f1,
		FileOut: f2,
	}

	ual.lock.Lock()
	defer ual.lock.Unlock()
	iid, err := ual.cual.Launch(helperAppId, helperExec, f1, f2)
	if err != nil {
		ual.log.Errorf("unable to launch helper %s: %v", helperAppId, err)
		return err
	}
	args.Id = iid
	args.Timer = time.AfterFunc(ual.maxRuntime, func() {
		ual.peekId(iid, func(a *HelperArgs) {
			a.ForcedStop = true
			err := ual.cual.Stop(helperAppId, iid)
			if err != nil {
				ual.log.Errorf("unable to forcefully stop helper %s: %v", helperAppId, err)
			}
		})
	})
	ual.hmap[iid] = &args

	return nil
}

func (ual *ualHelperLauncher) peekId(iid string, cb func(*HelperArgs)) *HelperArgs {
	ual.lock.Lock()
	defer ual.lock.Unlock()
	args, ok := ual.hmap[iid]
	if ok {
		cb(args)
		return args
	}
	return nil
}

func (ual *ualHelperLauncher) OneDone(iid string) {
	args := ual.peekId(iid, func(a *HelperArgs) {
		a.Timer.Stop()
		// dealt with, remove it
		delete(ual.hmap, iid)
	})
	if args == nil {
		// nothing to do
		return
	}
	defer func() {
		ual.cleanupTempFiles(args.FileIn, args.FileOut)
	}()
	if args.ForcedStop {
		ual.failOne(args.Input)
		return
	}
	payload, err := ioutil.ReadFile(args.FileOut)
	if err != nil {
		ual.log.Errorf("unable to read output from helper: %v", err)
	} else {
		res := &HelperResult{Input: args.Input}
		err = json.Unmarshal(payload, &res.HelperOutput)
		if err != nil {
			ual.log.Debugf("failed to parse HelperOutput from helper output: %v", err)
		} else {
			ual.chOut <- res
		}
	}
	if err != nil {
		ual.failOne(args.Input)
	}
}

func (ual *ualHelperLauncher) createInputTempFile(input *HelperInput) (string, error) {
	f1, err := getTempFilename(input.App.Package)
	if err != nil {
		return "", err
	}
	return f1, ioutil.WriteFile(f1, input.Payload, os.ModeTemporary)
}

func (ual *ualHelperLauncher) createOutputTempFile(input *HelperInput) (string, error) {
	return getTempFilename(input.App.Package)
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
