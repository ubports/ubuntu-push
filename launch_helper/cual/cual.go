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
gboolean stop(gchar* app_id, gchar* iid);
*/
import "C"
import (
	"errors"
	"unsafe"

	"launchpad.net/ubuntu-push/logger"
)

func gstring(s string) *C.gchar {
	return (*C.gchar)(C.CString(s))
}

type HelperState interface {
	InstallObserver() error
	RemoveObserver() error
	Launch(appId string, exec string, f1 string, f2 string) (string, error)
	Stop(appId string, instanceId string) error
}

type UAL interface {
	OneDone(string)
}

type helperState struct {
	log logger.Logger
	ual UAL
}

//export helperDone
func helperDone(gp unsafe.Pointer, ciid *C.char) {
	hs := (*helperState)(gp)
	iid := C.GoString(ciid)
	hs.ual.OneDone(iid)
}

var (
	ErrCantObserve   = errors.New("can't add observer")
	ErrCantUnobserve = errors.New("can't remove observer")
	ErrCantLaunch    = errors.New("can't launch helper")
	ErrCantStop      = errors.New("can't stop helper")
)

func New(log logger.Logger, ual UAL) HelperState {
	return &helperState{log: log, ual: ual}
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

func (hs *helperState) Launch(appId, exec, f1, f2 string) (string, error) {
	// launch(...) takes over ownership of things passed in
	iid := C.GoString(C.launch(gstring(appId), gstring(exec), gstring(f1), gstring(f2), C.gpointer(hs)))
	if iid == "" {
		return "", ErrCantLaunch
	}
	return iid, nil
}

func (hs *helperState) Stop(appId, instanceId string) error {
	if C.stop(gstring(appId), gstring(instanceId)) != C.TRUE {
		return ErrCantStop
	}
	return nil
}
