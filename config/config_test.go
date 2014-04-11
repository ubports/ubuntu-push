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

package config

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	. "launchpad.net/gocheck"
)

func TestConfig(t *testing.T) { TestingT(t) }

type configSuite struct{}

var _ = Suite(&configSuite{})

type testConfig1 struct {
	A int
	B string
	C []string `json:"c_list"`
}

func (s *configSuite) TestReadConfig(c *C) {
	buf := bytes.NewBufferString(`{"a": 1, "b": "foo", "c_list": ["c", "d", "e"]}`)
	var cfg testConfig1
	err := ReadConfig(buf, &cfg)
	c.Check(err, IsNil)
	c.Check(cfg, DeepEquals, testConfig1{A: 1, B: "foo", C: []string{"c", "d", "e"}})
}

func checkError(c *C, config string, dest interface{}, expectedError string) {
	buf := bytes.NewBufferString(config)
	err := ReadConfig(buf, dest)
	c.Check(err, ErrorMatches, expectedError)
}

func (s *configSuite) TestReadConfigErrors(c *C) {
	var cfg testConfig1
	checkError(c, "", cfg, `destConfig not \*struct`)
	var i int
	checkError(c, "", &i, `destConfig not \*struct`)
	checkError(c, "", &cfg, `EOF`)
	checkError(c, `{"a": "1"}`, &cfg, `a: .*type int`)
	checkError(c, `{"b": "1"}`, &cfg, `missing a`)
	checkError(c, `{"A": "1"}`, &cfg, `missing a`)
	checkError(c, `{"a": 1, "b": "foo"}`, &cfg, `missing c_list`)
}

type testTimeDurationConfig struct {
	D ConfigTimeDuration
}

func (s *configSuite) TestReadConfigTimeDuration(c *C) {
	buf := bytes.NewBufferString(`{"d": "2s"}`)
	var cfg testTimeDurationConfig
	err := ReadConfig(buf, &cfg)
	c.Assert(err, IsNil)
	c.Check(cfg.D.TimeDuration(), Equals, 2*time.Second)
}

func (s *configSuite) TestReadConfigTimeDurationErrors(c *C) {
	var cfg testTimeDurationConfig
	checkError(c, `{"d": 1}`, &cfg, "d:.*type string")
	checkError(c, `{"d": "2"}`, &cfg, "d:.*missing unit.*")
}

type testHostPortConfig struct {
	H ConfigHostPort
}

func (s *configSuite) TestReadConfigHostPort(c *C) {
	buf := bytes.NewBufferString(`{"h": "127.0.0.1:9999"}`)
	var cfg testHostPortConfig
	err := ReadConfig(buf, &cfg)
	c.Assert(err, IsNil)
	c.Check(cfg.H.HostPort(), Equals, "127.0.0.1:9999")
}

func (s *configSuite) TestReadConfigHostPortErrors(c *C) {
	var cfg testHostPortConfig
	checkError(c, `{"h": 1}`, &cfg, "h:.*type string")
	checkError(c, `{"h": ""}`, &cfg, "h: missing port in address")
}

type testQueueSizeConfig struct {
	QS ConfigQueueSize
}

func (s *configSuite) TestReadConfigQueueSize(c *C) {
	buf := bytes.NewBufferString(`{"qS": 1}`)
	var cfg testQueueSizeConfig
	err := ReadConfig(buf, &cfg)
	c.Assert(err, IsNil)
	c.Check(cfg.QS.QueueSize(), Equals, uint(1))
}

func (s *configSuite) TestReadConfigQueueSizeErrors(c *C) {
	var cfg testQueueSizeConfig
	checkError(c, `{"qS": "x"}`, &cfg, "qS: .*type uint")
	checkError(c, `{"qS": 0}`, &cfg, "qS: queue size should be > 0")
}

func (s *configSuite) TestLoadFile(c *C) {
	tmpDir := c.MkDir()
	d, err := LoadFile("", tmpDir)
	c.Check(err, IsNil)
	c.Check(d, IsNil)
	fullPath := filepath.Join(tmpDir, "example.file")
	err = ioutil.WriteFile(fullPath, []byte("Example"), os.ModePerm)
	c.Assert(err, IsNil)
	d, err = LoadFile("example.file", tmpDir)
	c.Check(err, IsNil)
	c.Check(string(d), Equals, "Example")
	d, err = LoadFile(fullPath, tmpDir)
	c.Check(err, IsNil)
	c.Check(string(d), Equals, "Example")
}

func (s *configSuite) TestReadFiles(c *C) {
	tmpDir := c.MkDir()
	cfg1Path := filepath.Join(tmpDir, "cfg1.json")
	err := ioutil.WriteFile(cfg1Path, []byte(`{"a": 42}`), os.ModePerm)
	c.Assert(err, IsNil)
	cfg2Path := filepath.Join(tmpDir, "cfg2.json")
	err = ioutil.WriteFile(cfg2Path, []byte(`{"b": "x", "c_list": ["y", "z"]}`), os.ModePerm)
	c.Assert(err, IsNil)
	var cfg testConfig1
	err = ReadFiles(&cfg, cfg1Path, cfg2Path)
	c.Assert(err, IsNil)
	c.Check(cfg.A, Equals, 42)
	c.Check(cfg.B, Equals, "x")
	c.Check(cfg.C, DeepEquals, []string{"y", "z"})
}

func (s *configSuite) TestReadFilesErrors(c *C) {
	var cfg testConfig1
	err := ReadFiles(1)
	c.Check(err, ErrorMatches, `destConfig not \*struct`)
	err = ReadFiles(&cfg, "non-existent")
	c.Check(err, ErrorMatches, "no config to read")
	err = ReadFiles(&cfg, "/root")
	c.Check(err, ErrorMatches, ".*permission denied")
	tmpDir := c.MkDir()
	err = ReadFiles(&cfg, tmpDir)
	c.Check(err, ErrorMatches, ".*is a directory")
	brokenCfgPath := filepath.Join(tmpDir, "b.json")
	err = ioutil.WriteFile(brokenCfgPath, []byte(`{"a"-`), os.ModePerm)
	c.Assert(err, IsNil)
	err = ReadFiles(&cfg, brokenCfgPath)
	c.Check(err, NotNil)
}

type B struct {
	BFld int
}

type A struct {
	AFld int
	B
	private int
}

func (s *configSuite) TestTraverseStruct(c *C) {
	var a A
	var i = 1
	for destField := range traverseStruct(reflect.ValueOf(&a).Elem()) {
		*(destField.dest.(*int)) = i
		i++
	}
	c.Check(a, DeepEquals, A{1, B{2}, 0})
}

type testConfig2 struct {
	A int
	B string
	C []string `json:"c_list"`
	D ConfigTimeDuration
}

func (s *configSuite) TestCompareConfig(c *C) {
	var cfg1 = testConfig2{
		A: 1,
		B: "xyz",
		C: []string{"a", "b"},
		D: ConfigTimeDuration{200 * time.Millisecond},
	}
	var cfg2 = testConfig2{
		A: 1,
		B: "xyz",
		C: []string{"a", "b"},
		D: ConfigTimeDuration{200 * time.Millisecond},
	}
	_, err := CompareConfig(cfg1, &cfg2)
	c.Check(err, ErrorMatches, `config1 not \*struct`)
	_, err = CompareConfig(&cfg1, cfg2)
	c.Check(err, ErrorMatches, `config2 not \*struct`)
	_, err = CompareConfig(&cfg1, &testConfig1{})
	c.Check(err, ErrorMatches, `config1 and config2 don't have the same type`)

	res, err := CompareConfig(&cfg1, &cfg2)
	c.Assert(err, IsNil)
	c.Check(res, IsNil)

	cfg1.B = "zyx"
	cfg2.C = []string{"a", "B"}
	cfg2.D = ConfigTimeDuration{205 * time.Millisecond}

	res, err = CompareConfig(&cfg1, &cfg2)
	c.Assert(err, IsNil)
	c.Check(res, DeepEquals, []string{"b", "c_list", "d"})

}

type testConfig3 struct {
	A bool
	B string
	C []string           `json:"c_list"`
	D ConfigTimeDuration `help:"duration"`
	E ConfigHostPort
	F string
}

type configFlagsSuite struct{}

var _ = Suite(&configFlagsSuite{})

func (s *configFlagsSuite) SetUpTest(c *C) {
	flag.CommandLine = flag.NewFlagSet("cmd", flag.PanicOnError)
	// supress outputs
	flag.Usage = func() { flag.PrintDefaults() }
	flag.CommandLine.SetOutput(ioutil.Discard)
}

func (s *configFlagsSuite) TestReadUsingFlags(c *C) {
	os.Args = []string{"cmd", "-a=0", "-b=foo", "-c_list", `["x","y"]`, "-d", "10s", "-e=localhost:80"}
	var cfg testConfig3
	p := make(map[string]json.RawMessage)
	err := readUsingFlags(p, reflect.ValueOf(&cfg))
	c.Assert(err, IsNil)
	c.Check(p, DeepEquals, map[string]json.RawMessage{
		"a":      json.RawMessage("false"),
		"b":      json.RawMessage(`"foo"`),
		"c_list": json.RawMessage(`["x","y"]`),
		"d":      json.RawMessage(`"10s"`),
		"e":      json.RawMessage(`"localhost:80"`),
	})
}

func (s *configFlagsSuite) TestReadUsingFlagsError(c *C) {
	os.Args = []string{"cmd", "-a=zoo"}
	var cfg testConfig3
	p := make(map[string]json.RawMessage)
	c.Check(func() { readUsingFlags(p, reflect.ValueOf(&cfg)) }, PanicMatches, ".*invalid boolean.*-a.*")
}

func (s *configFlagsSuite) TestReadFilesAndFlags(c *C) {
	// test <flags> pseudo file
	os.Args = []string{"cmd", "-a=42"}
	tmpDir := c.MkDir()
	cfgPath := filepath.Join(tmpDir, "cfg.json")
	err := ioutil.WriteFile(cfgPath, []byte(`{"b": "x", "c_list": ["y", "z"]}`), os.ModePerm)
	c.Assert(err, IsNil)
	var cfg testConfig1
	err = ReadFiles(&cfg, cfgPath, "<flags>")
	c.Assert(err, IsNil)
	c.Check(cfg.A, Equals, 42)
	c.Check(cfg.B, Equals, "x")
	c.Check(cfg.C, DeepEquals, []string{"y", "z"})
}

func (s *configFlagsSuite) TestReadFilesAndFlagsConfigAtSupport(c *C) {
	// test <flags> pseudo file
	tmpDir := c.MkDir()
	cfgPath := filepath.Join(tmpDir, "cfg.json")
	os.Args = []string{"cmd", "-a=42", fmt.Sprintf("-cfg@=%s", cfgPath)}
	err := ioutil.WriteFile(cfgPath, []byte(`{"b": "x", "c_list": ["y", "z"]}`), os.ModePerm)
	c.Assert(err, IsNil)
	var cfg testConfig1
	err = ReadFiles(&cfg, "<flags>")
	c.Assert(err, IsNil)
	c.Check(cfg.A, Equals, 42)
	c.Check(cfg.B, Equals, "x")
	c.Check(cfg.C, DeepEquals, []string{"y", "z"})
}

func (s *configFlagsSuite) TestReadUsingFlagsHelp(c *C) {
	os.Args = []string{"cmd", "-h"}
	buf := bytes.NewBufferString("")
	flag.CommandLine.Init("cmd", flag.ContinueOnError)
	flag.CommandLine.SetOutput(buf)
	var cfg testConfig3
	p := make(map[string]json.RawMessage)
	readUsingFlags(p, reflect.ValueOf(&cfg))
	c.Check(buf.String(), Matches, "(?s).*-cfg@=<config.json>: get config values from file\n.*-d.*duration.*")
}

func (s *configFlagsSuite) TestReadUsingFlagsAlreadyParsed(c *C) {
	os.Args = []string{"cmd"}
	flag.Parse()
	var cfg struct{}
	p := make(map[string]json.RawMessage)
	err := readUsingFlags(p, reflect.ValueOf(&cfg))
	c.Assert(err, ErrorMatches, "too late, flags already parsed")
	err = ReadFiles(&cfg, "<flags>")
	c.Assert(err, ErrorMatches, "too late, flags already parsed")
	IgnoreParsedFlags = true
	defer func() {
		IgnoreParsedFlags = false
	}()
	err = ReadFiles(&cfg, "<flags>")
	c.Assert(err, IsNil)
}
