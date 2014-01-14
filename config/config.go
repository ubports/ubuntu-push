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

// Package config has helpers to parse and use JSON based configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

// ReadConfig reads a JSON configuration into destConfig which should
// be a pointer to a structure, it does some more configuration
// specific error checking than plain JSON decoding and mentions
// fields in errors . Configuration fields are expected to start with
// lower case in the JSON object.
func ReadConfig(r io.Reader, destConfig interface{}) error {
	destValue := reflect.ValueOf(destConfig)
	if destValue.Kind() != reflect.Ptr || destValue.Elem().Kind() != reflect.Struct {
		return errors.New("destConfig not *struct")
	}
	// do the parsing in two phases for better error handling
	var p1 map[string]json.RawMessage
	err := json.NewDecoder(r).Decode(&p1)
	if err != nil {
		return err
	}
	destStruct := destValue.Elem()
	structType := destStruct.Type()
	n := structType.NumField()
	for i := 0; i < n; i++ {
		fld := structType.Field(i)
		configName := strings.Split(fld.Tag.Get("json"), ",")[0]
		if configName == "" {
			configName = strings.ToLower(fld.Name[:1]) + fld.Name[1:]
		}
		raw, found := p1[configName]
		if !found { // assume all fields are mandatory for now
			return fmt.Errorf("missing %s", configName)
		}
		dest := destStruct.Field(i).Addr().Interface()
		err = json.Unmarshal([]byte(raw), dest)
		if err != nil {
			return fmt.Errorf("%s: %v", configName, err)
		}
	}
	return nil
}

// ConfigTimeDuration can hold a time.Duration in a configuration struct,
// that is parsed from a string as supported by time.ParseDuration.
type ConfigTimeDuration struct {
	time.Duration
}

func (ctd *ConfigTimeDuration) UnmarshalJSON(b []byte) error {
	var enc string
	var v time.Duration
	err := json.Unmarshal(b, &enc)
	if err != nil {
		return err
	}
	v, err = time.ParseDuration(enc)
	if err != nil {
		return err
	}
	*ctd = ConfigTimeDuration{v}
	return nil
}

// TimeDuration returns the time.Duration held in ctd.
func (ctd ConfigTimeDuration) TimeDuration() time.Duration {
	return ctd.Duration
}

// ConfigHostPort can hold a host:port string in a configuration struct.
type ConfigHostPort string

func (chp *ConfigHostPort) UnmarshalJSON(b []byte) error {
	var enc string
	err := json.Unmarshal(b, &enc)
	if err != nil {
		return err
	}
	_, _, err = net.SplitHostPort(enc)
	if err != nil {
		return err
	}
	*chp = ConfigHostPort(enc)
	return nil
}

// HostPort returns the host:port string held in chp.
func (chp ConfigHostPort) HostPort() string {
	return string(chp)
}

// ConfigQueueSize can hold a queue size in a configuration struct.
type ConfigQueueSize uint

func (cqs *ConfigQueueSize) UnmarshalJSON(b []byte) error {
	var enc uint
	err := json.Unmarshal(b, &enc)
	if err != nil {
		return err
	}
	if enc == 0 {
		return errors.New("queue size should be > 0")
	}
	*cqs = ConfigQueueSize(enc)
	return nil
}

// QueueSize returns the queue size held in cqs.
func (cqs ConfigQueueSize) QueueSize() uint {
	return uint(cqs)
}

// LoadFile reads  a file possibly relative to a base dir.
func LoadFile(p, baseDir string) ([]byte, error) {
	if p == "" {
		return nil, nil
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(baseDir, p)
	}
	return ioutil.ReadFile(p)
}
