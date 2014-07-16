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

package cual

/*
#cgo pkg-config: ubuntu-app-launch-2
#include <glib.h>

gboolean add_observer (gpointer);
gboolean remove_observer (gpointer);
char* launch(gchar* app_id, gchar* exec, gchar* f1, gchar* f2, gpointer p);
void stop(gchar* app_id, gchar* iid);
*/
import "C"
import (
	"encoding/json"
	"errors"
	"sync"
	"time"
	"unsafe"

	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/logger"
)

func gstring(s string) *C.gchar {
	return (*C.gchar)(C.CString(s))
}

type HelperState interface {
	InstallObserver() error
	RemoveObserver() error
	Launch(*HelperArgs) error
}

type helperState struct {
	log  logger.Logger
	iids map[string]*HelperArgs
	lock sync.Mutex
}

type HelperArgs struct {
	App            *click.AppId
	NotificationId string
	Payload        json.RawMessage
	FileIn         string
	FileOut        string
	OneDone        func(*HelperArgs)
	timer          *time.Timer
}

//export helperDone
func helperDone(gp unsafe.Pointer, ciid *C.char) {
	hs := (*helperState)(gp)
	hs.lock.Lock()
	iid := C.GoString(ciid)
	hs.log.Debugf("helper %s stopped", iid)
	args, ok := hs.iids[iid]
	if !ok {
		return
	}
	delete(hs.iids, iid)
	args.timer.Stop()
	hs.lock.Unlock()
	args.OneDone(args)
}

var (
	ErrCantObserve    = errors.New("can't add observer")
	ErrCantUnobserve  = errors.New("can't remove observer")
	ErrCantFindHelper = errors.New("can't find helper")
)

func New(log logger.Logger) HelperState {
	return &helperState{log: log, iids: make(map[string]*HelperArgs)}
}

func (hs *helperState) InstallObserver() error {
	if C.add_observer(C.gpointer(hs)) != C.TRUE {
		return ErrCantObserve
	}
	return nil
}

func (hs *helperState) RemoveObserver() error {
	if C.remove_observer(C.gpointer(hs)) != C.TRUE {
		return ErrCantUnobserve
	}
	return nil
}

func (hs *helperState) Launch(args *HelperArgs) error {
	hs.lock.Lock()
	defer hs.lock.Unlock()
	helperAppId, helperExec := args.App.Helper()
	if helperAppId == "" || helperExec == "" {
		hs.log.Errorf("can't locate helper for app")
		return ErrCantFindHelper
	}
	hs.log.Debugf("using helper %s (exec: %s) for app %s", helperAppId, helperExec, args.App)
	// launch(...) takes over ownership of things passed in
	iid := C.GoString(C.launch(gstring(helperAppId), gstring(helperExec), gstring(args.FileIn), gstring(args.FileOut), C.gpointer(hs)))
	hs.iids[iid] = args
	args.timer = time.AfterFunc(5*time.Second, func() {
		hs.log.Debugf("timeout waiting for %s", iid)
		hs.lock.Lock()
		defer hs.lock.Unlock()
		_, ok := hs.iids[iid]
		if ok {
			// stop(...) takes over ownership of things passed in
			C.stop(gstring(helperAppId), gstring(iid))
			delete(hs.iids, iid)
		}
	})

	return nil
}
