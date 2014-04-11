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
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func checkDestConfig(name string, destConfig interface{}) (reflect.Value, error) {
	destValue := reflect.ValueOf(destConfig)
	if destValue.Kind() != reflect.Ptr || destValue.Elem().Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("%s not *struct", name)
	}
	return destValue, nil
}

type destField struct {
	fld  reflect.StructField
	dest interface{}
}

func (f destField) configName() string {
	fld := f.fld
	configName := strings.Split(fld.Tag.Get("json"), ",")[0]
	if configName == "" {
		configName = strings.ToLower(fld.Name[:1]) + fld.Name[1:]
	}
	return configName
}

func traverseStruct(destStruct reflect.Value) <-chan destField {
	ch := make(chan destField)
	var traverse func(reflect.Value, chan<- destField)
	traverse = func(destStruct reflect.Value, ch chan<- destField) {
		structType := destStruct.Type()
		n := structType.NumField()
		for i := 0; i < n; i++ {
			fld := structType.Field(i)
			val := destStruct.Field(i)
			if fld.PkgPath != "" { // unexported
				continue
			}
			if fld.Anonymous {
				traverse(val, ch)
				continue
			}
			ch <- destField{
				fld:  fld,
				dest: val.Addr().Interface(),
			}
		}
	}
	go func() {
		traverse(destStruct, ch)
		close(ch)
	}()
	return ch
}

func fillDestConfig(destValue reflect.Value, p map[string]json.RawMessage) error {
	destStruct := destValue.Elem()
	for destField := range traverseStruct(destStruct) {
		configName := destField.configName()
		raw, found := p[configName]
		if !found { // assume all fields are mandatory for now
			return fmt.Errorf("missing %s", configName)
		}
		dest := destField.dest
		err := json.Unmarshal([]byte(raw), dest)
		if err != nil {
			return fmt.Errorf("%s: %v", configName, err)
		}
	}
	return nil
}

// ReadConfig reads a JSON configuration into destConfig which should
// be a pointer to a structure. It does some more configuration
// specific error checking than plain JSON decoding, and mentions
// fields in errors. Configuration fields in the JSON object are
// expected to start with lower case.
func ReadConfig(r io.Reader, destConfig interface{}) error {
	destValue, err := checkDestConfig("destConfig", destConfig)
	if err != nil {
		return err
	}
	// do the parsing in two phases for better error handling
	var p1 map[string]json.RawMessage
	err = json.NewDecoder(r).Decode(&p1)
	if err != nil {
		return err
	}
	return fillDestConfig(destValue, p1)
}

// FromJSONString is for marking config types that are represented as
// strings originally in the JSON.
type FromJSONString interface {
	ConfigFromJSONString()
}

// ConfigTimeDuration can hold a time.Duration in a configuration struct,
// that is parsed from a string as supported by time.ParseDuration.
type ConfigTimeDuration struct {
	time.Duration
}

func (ctd *ConfigTimeDuration) ConfigFromJSONString() {}

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

func (chp *ConfigHostPort) ConfigFromJSONString() {}

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

// LoadFile reads a file possibly relative to a base dir.
func LoadFile(p, baseDir string) ([]byte, error) {
	if p == "" {
		return nil, nil
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(baseDir, p)
	}
	return ioutil.ReadFile(p)
}

// used to implement getting config values with flag.Parse()
type val struct {
	treatment string // b(bool)|s(tringify) or empty
	staging   map[string]json.RawMessage
	name      string
}

func (v *val) String() string {
	return ""
}

func (v *val) IsBoolFlag() bool {
	return v.treatment == "b"
}

func (v *val) Set(s string) error {
	var raw json.RawMessage
	switch v.treatment {
	case "b":
		bit, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		b, err := json.Marshal(bit)
		if err != nil {
			return err
		}
		raw = json.RawMessage(b)
	case "s":
		b, err := json.Marshal(s)
		if err != nil {
			return err
		}
		raw = json.RawMessage(b)
	default:
		raw = json.RawMessage(s)
	}
	v.staging[v.name] = raw
	return nil
}

func readOneConfig(staging map[string]json.RawMessage, cfgPath string) error {
	r, err := os.Open(cfgPath)
	if err != nil {
		return err
	}
	defer r.Close()
	err = json.NewDecoder(r).Decode(&staging)
	if err != nil {
		return err
	}
	return nil
}

// used to implement -cfg@=
type readConfigAtVal struct {
	staging map[string]json.RawMessage
}

func (v *readConfigAtVal) String() string {
	return "<config.json>"
}

func (v *readConfigAtVal) Set(path string) error {
	return readOneConfig(v.staging, path)
}

// readUsingFlags gets config values from command line flags.
func readUsingFlags(staging map[string]json.RawMessage, destValue reflect.Value) error {
	if flag.Parsed() {
		return fmt.Errorf("too late, flags already parsed")
	}
	destStruct := destValue.Elem()
	for destField := range traverseStruct(destStruct) {
		configName := destField.configName()
		treatment := ""
		switch destField.fld.Type.Kind() {
		case reflect.Bool:
			treatment = "b"
		case reflect.String:
			treatment = "s"
		default:
			if _, ok := destField.dest.(FromJSONString); ok {
				treatment = "s"
			}
		}
		help := destField.fld.Tag.Get("help")
		flag.Var(&val{treatment, staging, configName}, configName, help)
	}
	flag.Var(&readConfigAtVal{staging}, "cfg@", "get config values from file")
	flag.Parse()
	return nil
}

// ReadFiles reads configuration from a set of files. <flags> can be
// used as a pseudo file-path, it will consider command line flags,
// invoking flag.Parse(). Among those -cfg@=<file> can be used to get
// further config values from file.
func ReadFiles(destConfig interface{}, cfgFpaths ...string) error {
	destValue, err := checkDestConfig("destConfig", destConfig)
	if err != nil {
		return err
	}
	// do the parsing in two phases for better error handling
	p1 := make(map[string]json.RawMessage)
	readOne := false
	for _, cfgPath := range cfgFpaths {
		if cfgPath == "<flags>" {
			err := readUsingFlags(p1, destValue)
			if err != nil {
				return err
			}
			readOne = true
			continue
		}
		if _, err := os.Stat(cfgPath); err == nil {
			err := readOneConfig(p1, cfgPath)
			if err != nil {
				return err
			}
			readOne = true
		}
	}
	if !readOne {
		return fmt.Errorf("no config to read")
	}
	return fillDestConfig(destValue, p1)
}

// CompareConfigs compares the two given configuration structures. It returns a list of differing fields or nil if the config contents are the same.
func CompareConfig(config1, config2 interface{}) ([]string, error) {
	v1, err := checkDestConfig("config1", config1)
	if err != nil {
		return nil, err
	}
	v2, err := checkDestConfig("config2", config2)
	if err != nil {
		return nil, err
	}
	if v1.Type() != v2.Type() {
		return nil, errors.New("config1 and config2 don't have the same type")
	}
	fields1 := traverseStruct(v1.Elem())
	fields2 := traverseStruct(v2.Elem())
	diff := make([]string, 0)
	for {
		d1 := <-fields1
		d2 := <-fields2
		if d1.dest == nil {
			break
		}
		if !reflect.DeepEqual(d1.dest, d2.dest) {
			diff = append(diff, d1.configName())
		}
	}
	if len(diff) != 0 {
		return diff, nil
	}
	return nil, nil
}
