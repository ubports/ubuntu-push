package helper_finder

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"launchpad.net/go-xdg/v0"

	"github.com/ubports/ubuntu-push/click"
	"github.com/ubports/ubuntu-push/logger"
)

type helperValue struct {
	HelperId string `json:"helper_id"`
	Exec     string `json:"exec"`
}

type hookFile struct {
	AppId string `json:"app_id"`
	Exec  string `json:"exec"`
}

var mapLock sync.Mutex
var helpersInfo = make(map[string]helperValue)
var helpersDataMtime time.Time

var helpersDataPath = filepath.Join(xdg.Data.Home(), "ubuntu-push-client", "helpers_data.json")
var hookPath = filepath.Join(xdg.Data.Home(), "ubuntu-push-client", "helpers")
var hookExt = ".json"

// helperFromHookfile figures out the app id and executable of the untrusted
// helper for this app.
func helperFromHookFile(app *click.AppId) (helperAppId string, helperExec string) {
	matches, err := filepath.Glob(filepath.Join(hookPath, app.Package+"_*"+hookExt))
	if err != nil {
		return "", ""
	}
	var v hookFile
	for _, m := range matches {
		abs, err := filepath.EvalSymlinks(m)
		if err != nil {
			continue
		}
		data, err := ioutil.ReadFile(abs)
		if err != nil {
			continue
		}
		err = json.Unmarshal(data, &v)
		if err != nil {
			continue
		}
		if v.Exec != "" && (v.AppId == "" || v.AppId == app.Base()) {
			basename := filepath.Base(m)
			helperAppId = basename[:len(basename)-len(hookExt)]
			helperExec = filepath.Join(filepath.Dir(abs), v.Exec)
			return helperAppId, helperExec
		}
	}
	return "", ""
}

// Helper figures out the id and executable of the untrusted
// helper for this app.
func Helper(app *click.AppId, log logger.Logger) (helperAppId string, helperExec string) {
	if !app.Click {
		return "", ""
	}
	fInfo, err := os.Stat(helpersDataPath)
	if err != nil {
		// cache file is missing, go via the slow route
		log.Infof("cache file not found, falling back to .json file lookup")
		return helperFromHookFile(app)
	}
	// get the lock as the map can be changed while we read
	mapLock.Lock()
	defer mapLock.Unlock()
	if helpersInfo == nil || fInfo.ModTime().After(helpersDataMtime) {
		data, err := ioutil.ReadFile(helpersDataPath)
		if err != nil {
			return "", ""
		}
		err = json.Unmarshal(data, &helpersInfo)
		if err != nil {
			return "", ""
		}
		helpersDataMtime = fInfo.ModTime()
	}
	var info helperValue
	info, ok := helpersInfo[app.Base()]
	if !ok {
		// ok, appid wasn't there, try with the package
		info, ok = helpersInfo[app.Package]
		if !ok {
			return "", ""
		}
	}
	if info.Exec != "" {
		helperAppId = info.HelperId
		helperExec = info.Exec
		return helperAppId, helperExec
	}
	return "", ""
}
