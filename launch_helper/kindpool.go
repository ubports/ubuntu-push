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

	xdg "launchpad.net/go-xdg/v0"

	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/launch_helper/cual"
	"launchpad.net/ubuntu-push/launch_helper/legacy"
	"launchpad.net/ubuntu-push/logger"
)

var (
	ErrCantFindHelper   = errors.New("can't find helper")
	ErrCantFindLauncher = errors.New("can't find launcher for helper")
)

type HelperArgs struct {
	Input      *HelperInput
	AppId      string
	FileIn     string
	FileOut    string
	Timer      *time.Timer
	ForcedStop bool
}

type HelperLauncher interface {
	HelperInfo(app *click.AppId) (string, string)
	InstallObserver(done func(string)) error
	RemoveObserver() error
	Launch(appId string, exec string, f1 string, f2 string) (string, error)
	Stop(appId string, instanceId string) error
}

type kindHelperPool struct {
	log        logger.Logger
	chOut      chan *HelperResult
	chIn       chan *HelperInput
	chDone     chan *click.AppId
	chStopped  chan struct{}
	launchers  map[string]HelperLauncher
	lock       sync.Mutex
	hmap       map[string]*HelperArgs
	maxRuntime time.Duration
	maxNum     int
	// hook
	growBacklog func([]*HelperInput, *HelperInput) []*HelperInput
}

// DefaultLaunchers produces the default map for kind -> HelperLauncher
func DefaultLaunchers(log logger.Logger) map[string]HelperLauncher {
	return map[string]HelperLauncher{
		"click":  cual.New(log),
		"legacy": legacy.New(log),
	}
}

// a HelperPool that delegates to different per kind HelperLaunchers
func NewHelperPool(launchers map[string]HelperLauncher, log logger.Logger) HelperPool {
	newPool := &kindHelperPool{
		log:        log,
		hmap:       make(map[string]*HelperArgs),
		launchers:  launchers,
		maxRuntime: 5 * time.Second,
		maxNum:     5,
	}
	newPool.growBacklog = newPool.doGrowBacklog
	return newPool
}

func (pool *kindHelperPool) Start() chan *HelperResult {
	pool.chOut = make(chan *HelperResult)
	pool.chIn = make(chan *HelperInput, InputBufferSize)
	pool.chDone = make(chan *click.AppId)
	pool.chStopped = make(chan struct{})

	for kind, launcher := range pool.launchers {
		kind1 := kind
		err := launcher.InstallObserver(func(iid string) {
			pool.OneDone(kind1 + ":" + iid)
		})
		if err != nil {
			panic(fmt.Errorf("failed to install helper observer for %s: %v", kind, err))
		}
	}

	go pool.loop()

	return pool.chOut
}

func (pool *kindHelperPool) loop() {
	running := make(map[string]bool)
	var backlog []*HelperInput

	for {
		select {
		case in, ok := <-pool.chIn:
			if !ok {
				close(pool.chStopped)
				return
			}
			if len(running) >= pool.maxNum || running[in.App.Original()] {
				backlog = pool.growBacklog(backlog, in)
			} else {
				if pool.tryOne(in) {
					running[in.App.Original()] = true
				}
			}
		case app := <-pool.chDone:
			delete(running, app.Original())
			if len(backlog) == 0 {
				continue
			}
			backlogSz := 0
			done := false
			for i, in := range backlog {
				if in != nil {
					if !done && !running[in.App.Original()] {
						backlog[i] = nil
						if pool.tryOne(in) {
							running[in.App.Original()] = true
							done = true
						}
					} else {
						backlogSz++
					}
				}
			}
			backlog = pool.shrinkBacklog(backlog, backlogSz)
			pool.log.Debugf("current helper input backlog has shrunk to %d entries.", backlogSz)
		}
	}
}

func (pool *kindHelperPool) doGrowBacklog(backlog []*HelperInput, in *HelperInput) []*HelperInput {
	backlog = append(backlog, in)
	pool.log.Debugf("current helper input backlog has grown to %d entries.", len(backlog))
	return backlog
}

func (pool *kindHelperPool) shrinkBacklog(backlog []*HelperInput, backlogSz int) []*HelperInput {
	if backlogSz == 0 {
		return nil
	}
	if cap(backlog) < 2*backlogSz {
		return backlog
	}
	pool.log.Debugf("copying backlog to avoid wasting too much space (%d/%d used)", backlogSz, cap(backlog))
	clean := make([]*HelperInput, 0, backlogSz)
	for _, bentry := range backlog {
		if bentry != nil {
			clean = append(clean, bentry)
		}
	}
	return clean
}

func (pool *kindHelperPool) Stop() {
	close(pool.chIn)
	for kind, launcher := range pool.launchers {
		err := launcher.RemoveObserver()
		if err != nil {
			panic(fmt.Errorf("failed to remove helper observer for &s: %v", kind, err))
		}
	}
	// make Stop sync for tests
	<-pool.chStopped
}

func (pool *kindHelperPool) Run(kind string, input *HelperInput) {
	input.kind = kind
	pool.chIn <- input
}

func (pool *kindHelperPool) tryOne(input *HelperInput) bool {
	if pool.handleOne(input) != nil {
		pool.failOne(input)
		return false
	}
	return true
}

func (pool *kindHelperPool) failOne(input *HelperInput) {
	pool.log.Errorf("unable to get helper output; putting payload into message")
	pool.chOut <- &HelperResult{HelperOutput: HelperOutput{Message: input.Payload, Notification: nil}, Input: input}
}

func (pool *kindHelperPool) cleanupTempFiles(f1, f2 string) {
	if f1 != "" {
		os.Remove(f1)
	}
	if f2 != "" {
		os.Remove(f2)
	}
}

func (pool *kindHelperPool) handleOne(input *HelperInput) error {
	launcher, ok := pool.launchers[input.kind]
	if !ok {
		pool.log.Errorf("unable to find launcher for kind: %v", input.kind)
		return ErrCantFindLauncher
	}
	helperAppId, helperExec := launcher.HelperInfo(input.App)
	if helperAppId == "" && helperExec == "" {
		pool.log.Errorf("can't locate helper for app")
		return ErrCantFindHelper
	}
	pool.log.Debugf("using helper %s (exec: %s) for app %s", helperAppId, helperExec, input.App)
	var f1, f2 string
	f1, err := pool.createInputTempFile(input)
	defer func() {
		if err != nil {
			pool.cleanupTempFiles(f1, f2)
		}
	}()
	if err != nil {
		pool.log.Errorf("unable to create input tempfile: %v", err)
		return err
	}
	f2, err = pool.createOutputTempFile(input)
	if err != nil {
		pool.log.Errorf("unable to create output tempfile: %v", err)
		return err
	}

	args := HelperArgs{
		AppId:   helperAppId,
		Input:   input,
		FileIn:  f1,
		FileOut: f2,
	}

	pool.lock.Lock()
	defer pool.lock.Unlock()
	iid, err := launcher.Launch(helperAppId, helperExec, f1, f2)
	if err != nil {
		pool.log.Errorf("unable to launch helper %s: %v", helperAppId, err)
		return err
	}
	uid := input.kind + ":" + iid // unique across launchers
	args.Timer = time.AfterFunc(pool.maxRuntime, func() {
		pool.peekId(uid, func(a *HelperArgs) {
			a.ForcedStop = true
			err := launcher.Stop(helperAppId, iid)
			if err != nil {
				pool.log.Errorf("unable to forcefully stop helper %s: %v", helperAppId, err)
			}
		})
	})
	pool.hmap[uid] = &args

	return nil
}

func (pool *kindHelperPool) peekId(uid string, cb func(*HelperArgs)) *HelperArgs {
	pool.lock.Lock()
	defer pool.lock.Unlock()
	args, ok := pool.hmap[uid]
	if ok {
		cb(args)
		return args
	}
	return nil
}

func (pool *kindHelperPool) OneDone(uid string) {
	args := pool.peekId(uid, func(a *HelperArgs) {
		a.Timer.Stop()
		// dealt with, remove it
		delete(pool.hmap, uid)
	})
	if args == nil {
		// nothing to do
		return
	}
	defer func() {
		pool.cleanupTempFiles(args.FileIn, args.FileOut)
	}()
	if args.ForcedStop {
		pool.failOne(args.Input)
		return
	}
	payload, err := ioutil.ReadFile(args.FileOut)
	if err != nil {
		pool.log.Errorf("unable to read output from %v helper: %v", args.AppId, err)
	} else {
		pool.log.Infof("%v helper output: %s", args.AppId, payload)
		res := &HelperResult{Input: args.Input}
		err = json.Unmarshal(payload, &res.HelperOutput)
		if err != nil {
			pool.log.Errorf("failed to parse HelperOutput from %v helper output: %v", args.AppId, err)
		} else {
			pool.chOut <- res
		}
	}
	if err != nil {
		pool.failOne(args.Input)
	}
}

func (pool *kindHelperPool) createInputTempFile(input *HelperInput) (string, error) {
	f1, err := getTempFilename(input.App.Package)
	if err != nil {
		return "", err
	}
	return f1, ioutil.WriteFile(f1, input.Payload, os.ModeTemporary)
}

func (pool *kindHelperPool) createOutputTempFile(input *HelperInput) (string, error) {
	return getTempFilename(input.App.Package)
}

// helper helpers:

var xdgCacheHome = xdg.Cache.Home

func _getTempDir(pkgName string) (string, error) {
	tmpDir := path.Join(xdgCacheHome(), pkgName)
	err := os.MkdirAll(tmpDir, 0700)
	return tmpDir, err
}

// override GetTempDir for testing without writing to ~/.cache/<pkgName>
var GetTempDir func(pkgName string) (string, error) = _getTempDir

func _getTempFilename(pkgName string) (string, error) {
	tmpDir, err := GetTempDir(pkgName)
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
