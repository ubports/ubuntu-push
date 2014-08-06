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

package launch_helper

import (
	"encoding/json"
	"time"

	"launchpad.net/ubuntu-push/click"
)

// a Card is the usual “visual” presentation of a notification, used
// for bubbles and the notification centre (neé messaging menu)
type Card struct {
	Summary   string   `json:"summary"`   // required for the card to be presented
	Body      string   `json:"body"`      // defaults to empty
	Actions   []string `json:"actions"`   // if empty (default), bubble is non-clickable. More entries change it to be clickable and (for bubbles) snap-decisions.
	Icon      string   `json:"icon"`      // an icon relating to the event being notified. Defaults to empty (no icon); a secondary icon relating to the application will be shown as well, irrespectively.
	Timestamp int      `json:"timestamp"` // seconds since epoch, only used for persist (for now)
	Persist   bool     `json:"persist"`   // whether to show in notification centre; defaults to false
	Popup     bool     `json:"popup"`     // whether to show in a bubble. Users can disable this, and can easily miss them, so don't rely on it exclusively. Defaults to false.
}

// an EmblemCounter puts a number on an emblem on an app's icon in the launcher
type EmblemCounter struct {
	Count   int32 `json:"count"`   // the number to show on the emblem counter
	Visible bool  `json:"visible"` // whether to show the emblem counter
}

// a Vibration generates a vibration in the form of a Pattern set in
// duration a pattern of on off states, repeated a number of times
type Vibration struct {
	Duration uint32   `json:"duration"` // if Duration is present and not 0, it's like a Pattern of [Duration]; otherwise, Pattern is used.
	Pattern  []uint32 `json:"pattern"`
	Repeat   uint32   `json:"repeat"` // defaults to 1. A value of zero is ignored (so it's like 1).
}

// a Notification can be any of the above
type Notification struct {
	Card          *Card          `json:"card"`           // defaults to nil (no card)
	Sound         string         `json:"sound"`          // a sound file. Users can disable this, so don't rely on it exclusively. Defaults to empty (no sound).
	Vibrate       *Vibration     `json:"vibrate"`        // users can disable this, blah blah. Defaults to null (no vibration)
	EmblemCounter *EmblemCounter `json:"emblem-counter"` // puts a counter on an emblem in the launcher. Defaults to nil (no change to emblem counter).
	Tag           string         `json:"tag,omitempty"`  // tag used for Clear/ListPersistent.
}

// HelperOutput is the expected output of a helper
type HelperOutput struct {
	Message      json.RawMessage `json:"message,omitempty"`      // what to put in the post office's queue
	Notification *Notification   `json:"notification,omitempty"` // what to present to the user
}

// HelperResult is the result of a helper run for a particular app id
type HelperResult struct {
	HelperOutput
	Input *HelperInput
}

// HelperInput is what's passed in to a helper for it to work
type HelperInput struct {
	kind           string
	App            *click.AppId
	NotificationId string
	Payload        json.RawMessage
}

type rawnotif struct {
	Card          *Card          `json:"card"`
	Sound         string         `json:"sound"`
	Vibrate       *Vibration     `json:"vibrate"`
	EmblemCounter *EmblemCounter `json:"emblem-counter"`
	Tag           string         `json:"tag,omitempty"`
}

func (n *Notification) UnmarshalJSON(b []byte) error {
	var raw rawnotif
	err := json.Unmarshal(b, &raw)
	if err != nil {
		return err
	}

	n.Card = raw.Card
	n.EmblemCounter = raw.EmblemCounter
	n.Tag = raw.Tag
	n.Sound = raw.Sound
	n.Vibrate = raw.Vibrate

	if n.Card != nil && n.Card.Timestamp == 0 {
		n.Card.Timestamp = int(time.Now().Unix())
	}

	return nil
}
