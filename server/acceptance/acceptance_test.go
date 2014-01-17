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

package acceptance

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/server/api"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestAcceptance(t *testing.T) { TestingT(t) }

type acceptanceSuite struct {
	server       *exec.Cmd
	serverAddr   string
	serverURL    string
	serverEvents <-chan string
	httpClient   *http.Client
}

var _ = Suite(&acceptanceSuite{})

var serverCmd = flag.String("server", "", "server to test")

// SourceRelative produces a path relative to the source code, makes
// sense only for tests when the code is available on disk.
// xxx later move it to a testing helpers package
func SourceRelative(relativePath string) string {
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		panic("failed to get source filename using Caller()")
	}
	return filepath.Join(filepath.Dir(file), relativePath)
}

func testServerConfig(addr, httpAddr string) map[string]interface{} {
	cfg := map[string]interface{}{
		"exchange_timeout":   "0.1s",
		"ping_interval":      "0.5s",
		"session_queue_size": 10,
		"broker_queue_size":  100,
		"addr":               addr,
		"key_pem_file":       SourceRelative("config/testing.key"),
		"cert_pem_file":      SourceRelative("config/testing.cert"),
		"http_addr":          httpAddr,
		"http_read_timeout":  "1s",
		"http_write_timeout": "1s",
	}
	return cfg
}

func testClientSession(addr string, deviceId string, reportPings bool) *ClientSession {
	certPEMBlock, err := ioutil.ReadFile(SourceRelative("config/testing.cert"))
	if err != nil {
		panic(fmt.Sprintf("could not read config/testing.cert: %v", err))
	}
	return &ClientSession{
		ExchangeTimeout: 100 * time.Millisecond,
		ServerAddr:      addr,
		CertPEMBlock:    certPEMBlock,
		DeviceId:        deviceId,
		ReportPings:     reportPings,
	}
}

const (
	devListeningOnPat  = "INFO listening for devices on "
	httpListeningOnPat = "INFO listening for http on "
	debugPrefix = "DEBUG "
)

var rxLineInfo = regexp.MustCompile("^.*? ([[:alpha:]].*)\n")

func extractListeningAddr(c *C, pat, line string) string {
	if !strings.HasPrefix(line, pat) {
		c.Fatalf("server: %v", line)
	}
	return line[len(pat):]
}

// start a new server for each test
func (s *acceptanceSuite) SetUpTest(c *C) {
	if *serverCmd == "" {
		c.Skip("executable server not specified")
	}
	tmpDir := c.MkDir()
	cfgFilename := filepath.Join(tmpDir, "config.json")
	cfgJson, err := json.Marshal(testServerConfig("127.0.0.1:0", "127.0.0.1:0"))
	if err != nil {
		c.Fatal(err)
	}
	err = ioutil.WriteFile(cfgFilename, cfgJson, os.ModePerm)
	if err != nil {
		c.Fatal(err)
	}
	server := exec.Command(*serverCmd, cfgFilename)
	stderr, err := server.StderrPipe()
	if err != nil {
		c.Fatal(err)
	}
	err = server.Start()
	if err != nil {
		c.Fatal(err)
	}
	bufErr := bufio.NewReaderSize(stderr, 5000)
	getLineInfo := func(ignoreDebug bool) (string, error) {
		for {
			line, err := bufErr.ReadString('\n')
			if err != nil {
				return "", err
			}
			extracted := rxLineInfo.FindStringSubmatch(line)
			if extracted == nil {
				return "", fmt.Errorf("unexpected server line: %#v", line)
			}
			info := extracted[1]
			if ignoreDebug && strings.HasPrefix(info, debugPrefix) {
				// don't report DEBUG lines
				continue
			}
			return info, nil
		}
	}
	infoHTTP, err := getLineInfo(true)
	if err != nil {
		c.Fatal(err)
	}
	serverHTTPAddr := extractListeningAddr(c, httpListeningOnPat, infoHTTP)
	s.serverURL = fmt.Sprintf("http://%s", serverHTTPAddr)
	info, err := getLineInfo(true)
	if err != nil {
		c.Fatal(err)
	}
	s.serverAddr = extractListeningAddr(c, devListeningOnPat, info)
	s.server = server
	serverEvents := make(chan string, 5)
	s.serverEvents = serverEvents
	go func() {
		for {
			info, err := getLineInfo(false)
			if err != nil {
				serverEvents <- fmt.Sprintf("ERROR: %v", err)
				close(serverEvents)
				return
			}
			serverEvents <- info
		}
	}()
	s.httpClient = &http.Client{}
}

func (s *acceptanceSuite) TearDownTest(c *C) {
	if s.server != nil {
		s.server.Process.Kill()
		s.server = nil
	}
}

// nextEvent receives an event from given channel with a 5s timeout
func nextEvent(events <-chan string, errCh <-chan error) string {
	select {
	case <-time.After(5 * time.Second):
		panic("too long stuck waiting for next event")
	case err := <-errCh:
		return err.Error() // will fail comparison typically
	case evStr := <-events:
		return evStr
	}
}

// Tests about connection, ping-pong, disconnection scenarios

// typically combined with -gocheck.vv or test selection
var logTraffic = flag.Bool("logTraffic", false, "log traffic")

type connInterceptor func(ic *interceptingConn, op string, b []byte) (bool, int, error)

type interceptingConn struct {
	net.Conn
	totalRead    int
	totalWritten int
	intercept    connInterceptor
}

func (ic *interceptingConn) Write(b []byte) (n int, err error) {
	done := false
	before := ic.totalWritten
	if ic.intercept != nil {
		done, n, err = ic.intercept(ic, "write", b)
	}
	if !done {
		n, err = ic.Conn.Write(b)
	}
	ic.totalWritten += n
	if *logTraffic {
		fmt.Printf("W[%v]: %d %#v %v %d\n", ic.Conn.LocalAddr(), before, string(b[:n]), err, ic.totalWritten)
	}
	return
}

func (ic *interceptingConn) Read(b []byte) (n int, err error) {
	done := false
	before := ic.totalRead
	if ic.intercept != nil {
		done, n, err = ic.intercept(ic, "read", b)
	}
	if !done {
		n, err = ic.Conn.Read(b)
	}
	ic.totalRead += n
	if *logTraffic {
		fmt.Printf("R[%v]: %d %#v %v %d\n", ic.Conn.LocalAddr(), before, string(b[:n]), err, ic.totalRead)
	}
	return
}

func (s *acceptanceSuite) TestConnectPingPing(c *C) {
	errCh := make(chan error, 1)
	events := make(chan string, 10)
	sess := testClientSession(s.serverAddr, "DEVA", true)
	err := sess.Dial()
	c.Assert(err, IsNil)
	intercept := func(ic *interceptingConn, op string, b []byte) (bool, int, error) {
		// would be 3rd ping read, based on logged traffic
		if op == "read" && ic.totalRead >= 79 {
			// exit the sess.Run() goroutine, client will close
			runtime.Goexit()
		}
		return false, 0, nil
	}
	sess.Connection = &interceptingConn{sess.Connection, 0, 0, intercept}
	go func() {
		errCh <- sess.Run(events)
	}()
	connectCli := nextEvent(events, errCh)
	connectSrv := nextEvent(s.serverEvents, nil)
	registeredSrv := nextEvent(s.serverEvents, nil)
	tconnect := time.Now()
	c.Assert(connectSrv, Matches, ".*session.* connected .*")
	c.Assert(registeredSrv, Matches, ".*session.* registered DEVA")
	c.Assert(strings.HasSuffix(connectSrv, connectCli), Equals, true)
	c.Assert(nextEvent(events, errCh), Equals, "Ping")
	elapsedOfPing := float64(time.Since(tconnect)) / float64(500*time.Millisecond)
	c.Check(elapsedOfPing >= 1.0, Equals, true)
	c.Check(elapsedOfPing < 1.05, Equals, true)
	c.Assert(nextEvent(events, errCh), Equals, "Ping")
	c.Assert(nextEvent(s.serverEvents, nil), Matches, ".*session.* ended with: EOF")
	c.Check(len(errCh), Equals, 0)
}

func (s *acceptanceSuite) TestConnectPingNeverPong(c *C) {
	errCh := make(chan error, 1)
	events := make(chan string, 10)
	sess := testClientSession(s.serverAddr, "DEVB", true)
	err := sess.Dial()
	c.Assert(err, IsNil)
	intercept := func(ic *interceptingConn, op string, b []byte) (bool, int, error) {
		// would be pong to 2nd ping, based on logged traffic
		if op == "write" && ic.totalRead >= 67 {
			time.Sleep(200 * time.Millisecond)
			// exit the sess.Run() goroutine, client will close
			runtime.Goexit()
		}
		return false, 0, nil
	}
	sess.Connection = &interceptingConn{sess.Connection, 0, 0, intercept}
	go func() {
		errCh <- sess.Run(events)
	}()
	c.Assert(nextEvent(events, errCh), Matches, "connected .*")
	c.Assert(nextEvent(s.serverEvents, nil), Matches, ".*session.* connected .*")
	c.Assert(nextEvent(s.serverEvents, nil), Matches, ".*session.* registered .*")
	c.Assert(nextEvent(events, errCh), Equals, "Ping")
	c.Assert(nextEvent(s.serverEvents, nil), Matches, `.* ended with:.*timeout`)
	c.Check(len(errCh), Equals, 0)
}

// Tests about broadcast

func (s *acceptanceSuite) postRequest(path string, message interface{}) (string, error) {
	packedMessage, err := json.Marshal(message)
	if err != nil {
		panic(err)
	}
	reader := bytes.NewReader(packedMessage)

	url := s.serverURL + path
	request, _ := http.NewRequest("POST", url, reader)
	request.ContentLength = int64(reader.Len())
	request.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(request)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return string(body), err
}

func (s *acceptanceSuite) startClient(c *C, devId string, intercept connInterceptor, levels map[string]int64) (<-chan string, <-chan error) {
	errCh := make(chan error, 1)
	events := make(chan string, 10)
	sess := testClientSession(s.serverAddr, devId, false)
	sess.Levels = levels
	err := sess.Dial()
	c.Assert(err, IsNil)
	sess.Connection = &interceptingConn{sess.Connection, 0, 0, intercept}
	go func() {
		errCh <- sess.Run(events)
	}()
	c.Assert(nextEvent(events, errCh), Matches, "connected .*")
	c.Assert(nextEvent(s.serverEvents, nil), Matches, ".*session.* connected .*")
	c.Assert(nextEvent(s.serverEvents, nil), Matches, ".*session.* registered "+devId)
	return events, errCh
}

func (s *acceptanceSuite) TestBroadcastToConnected(c *C) {
	clientShutdown := make(chan bool, 1) // abused as an atomic flag
	intercept := func(ic *interceptingConn, op string, b []byte) (bool, int, error) {
		// read after ack
		if op == "read" && len(clientShutdown) > 0 {
			// exit the sess.Run() goroutine, client will close
			runtime.Goexit()
		}
		return false, 0, nil
	}
	events, errCh := s.startClient(c, "DEVB", intercept, nil)
	got, err := s.postRequest("/broadcast", &api.Broadcast{
		Channel: "system",
		Data:    json.RawMessage(`{"n": 42}`),
	})
	c.Check(err, IsNil)
	c.Check(got, Matches, ".*ok.*")
	c.Check(nextEvent(events, errCh), Equals, `broadcast chan:0 app: topLevel:1 payloads:[{"n":42}]`)
	clientShutdown <- true
	c.Assert(nextEvent(s.serverEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
}

func (s *acceptanceSuite) TestBroadcastPending(c *C) {
	// send broadcast that will be pending
	got, err := s.postRequest("/broadcast", &api.Broadcast{
		Channel: "system",
		Data:    json.RawMessage(`{"b": 1}`),
	})
	c.Check(err, IsNil)
	c.Check(got, Matches, ".*ok.*")

	clientShutdown := make(chan bool, 1) // abused as an atomic flag
	intercept := func(ic *interceptingConn, op string, b []byte) (bool, int, error) {
		// read after ack
		if op == "read" && len(clientShutdown) > 0 {
			// exit the sess.Run() goroutine, client will close
			runtime.Goexit()
		}
		return false, 0, nil
	}
	events, errCh := s.startClient(c, "DEVB", intercept, nil)
	// gettting pending on connect
	c.Check(nextEvent(events, errCh), Equals, `broadcast chan:0 app: topLevel:1 payloads:[{"b":1}]`)
	clientShutdown <- true
	c.Assert(nextEvent(s.serverEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
}

func (s *acceptanceSuite) TestBroadcasLargeNeedsSplitting(c *C) {
	// send bunch of broadcasts that will be pending
	payloadFmt := fmt.Sprintf(`{"b":%%d,"bloat":"%s"}`, strings.Repeat("x", 1024*2))
	for i := 0; i < 32; i++ {
		got, err := s.postRequest("/broadcast", &api.Broadcast{
			Channel: "system",
			Data:    json.RawMessage(fmt.Sprintf(payloadFmt, i)),
		})
		c.Check(err, IsNil)
		c.Check(got, Matches, ".*ok.*")
	}

	clientShutdown := make(chan bool, 1) // abused as an atomic flag
	intercept := func(ic *interceptingConn, op string, b []byte) (bool, int, error) {
		// read after ack
		if op == "read" && len(clientShutdown) > 0 {
			// exit the sess.Run() goroutine, client will close
			runtime.Goexit()
		}
		return false, 0, nil
	}
	events, errCh := s.startClient(c, "DEVC", intercept, nil)
	// gettting pending on connect
	c.Check(nextEvent(events, errCh), Matches, `broadcast chan:0 app: topLevel:30 payloads:\[{"b":0,.*`)
	c.Check(nextEvent(events, errCh), Matches, `broadcast chan:0 app: topLevel:32 payloads:\[.*`)
	clientShutdown <- true
	c.Assert(nextEvent(s.serverEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
}

func (s *acceptanceSuite) TestBroadcastDistribution2(c *C) {
	clientShutdown := make(chan bool, 1) // abused as an atomic flag
	intercept := func(ic *interceptingConn, op string, b []byte) (bool, int, error) {
		// read after ack
		if op == "read" && len(clientShutdown) > 0 {
			// exit the sess.Run() goroutine, client will close
			runtime.Goexit()
		}
		return false, 0, nil
	}
	// start 1st clinet
	events1, errCh1 := s.startClient(c, "DEV1", intercept, nil)
	// start 2nd client
	events2, errCh2 := s.startClient(c, "DEV2", intercept, nil)
	// broadcast
	got, err := s.postRequest("/broadcast", &api.Broadcast{
		Channel: "system",
		Data:    json.RawMessage(`{"n": 42}`),
	})
	c.Check(err, IsNil)
	c.Check(got, Matches, ".*ok.*")
	c.Check(nextEvent(events1, errCh1), Equals, `broadcast chan:0 app: topLevel:1 payloads:[{"n":42}]`)
	c.Check(nextEvent(events2, errCh2), Equals, `broadcast chan:0 app: topLevel:1 payloads:[{"n":42}]`)
	clientShutdown <- true
	c.Assert(nextEvent(s.serverEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Assert(nextEvent(s.serverEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh1), Equals, 0)
	c.Check(len(errCh2), Equals, 0)
}

func (s *acceptanceSuite) TestBroadcastFilterByLevel(c *C) {
	clientShutdown := make(chan bool, 1) // abused as an atomic flag
	intercept := func(ic *interceptingConn, op string, b []byte) (bool, int, error) {
		// read after ack
		if op == "read" && len(clientShutdown) > 0 {
			// exit the sess.Run() goroutine, client will close
			runtime.Goexit()
		}
		return false, 0, nil
	}
	events, errCh := s.startClient(c, "DEVD", intercept, nil)
	got, err := s.postRequest("/broadcast", &api.Broadcast{
		Channel: "system",
		Data:    json.RawMessage(`{"b": 1}`),
	})
	c.Check(err, IsNil)
	c.Check(got, Matches, ".*ok.*")
	c.Check(nextEvent(events, errCh), Equals, `broadcast chan:0 app: topLevel:1 payloads:[{"b":1}]`)
	clientShutdown <- true
	c.Assert(nextEvent(s.serverEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
	// another broadcast
	got, err = s.postRequest("/broadcast", &api.Broadcast{
		Channel: "system",
		Data:    json.RawMessage(`{"b": 2}`),
	})
	c.Check(err, IsNil)
	c.Check(got, Matches, ".*ok.*")
	// reconnect, provide levels, get only later notification
	<-clientShutdown // reset
	events, errCh = s.startClient(c, "DEVD", intercept, map[string]int64{
		protocol.SystemChannelId: 1,
	})
	c.Check(nextEvent(events, errCh), Equals, `broadcast chan:0 app: topLevel:2 payloads:[{"b":2}]`)
	clientShutdown <- true
	c.Assert(nextEvent(s.serverEvents, nil), Matches, `.* ended with:.*EOF`)
	c.Check(len(errCh), Equals, 0)
}
