package emblemcounter

import (
	"launchpad.net/go-dbus/v1"

	"launchpad.net/ubuntu-push/bus"
	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/launch_helper"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/nih"
)

var BusAddress = bus.Address{
	Interface: "com.canonical.Unity.Launcher.Item",
	Path:      "/com/canonical/Unity/Launcher",
	Name:      "com.canonical.Unity.Launcher",
}

// EmblemCounter is a little tool that fiddles with the unity launcher
// to put emblems with counters on launcher icons.
type EmblemCounter struct {
	bus bus.Endpoint
	log logger.Logger
}

// Build an EmblemCounter using the given bus and log.
func New(endp bus.Endpoint, log logger.Logger) *EmblemCounter {
	return &EmblemCounter{bus: endp, log: log}
}

// Look for an EmblemCounter section in a Notification and, if
// present, present it to the user.
func (ctr *EmblemCounter) Present(appId string, notificationId string, notification *launch_helper.Notification) {
	if notification == nil || notification.EmblemCounter == nil {
		ctr.log.Debugf("no notification or no EmblemCounter in the notification; doing nothing: %#v", notification)
		return
	}
	parsed, err := click.ParseAppId(appId)
	if err != nil {
		ctr.log.Debugf("no appId in %#v", appId)
		return
	}
	ec := notification.EmblemCounter
	ctr.log.Debugf("setting emblem counter for %s to %d (visible: %t)", appId, ec.Count, ec.Visible)

	quoted := string(nih.Quote([]byte(parsed.Application)))

	ctr.bus.SetProperty("count", "/"+quoted, dbus.Variant{ec.Count})
	ctr.bus.SetProperty("countVisible", "/"+quoted, dbus.Variant{ec.Visible})
}
