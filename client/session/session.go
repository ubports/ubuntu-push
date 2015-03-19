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

// Package session handles the minutiae of interacting with
// the Ubuntu Push Notifications server.
package session

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"launchpad.net/ubuntu-push/click"
	"launchpad.net/ubuntu-push/client/gethosts"
	"launchpad.net/ubuntu-push/client/session/seenstate"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/util"
)

type sessCmd uint8

const (
	cmdDisconnect sessCmd = iota
	cmdConnect
	cmdResetCookie
)

var (
	wireVersionBytes = []byte{protocol.ProtocolWireVersion}
)

type BroadcastNotification struct {
	TopLevel int64
	Decoded  []map[string]interface{}
}

type serverMsg struct {
	Type string `json:"T"`
	protocol.BroadcastMsg
	protocol.NotificationsMsg
	protocol.ConnBrokenMsg
	protocol.SetParamsMsg
}

// parseServerAddrSpec recognizes whether spec is a HTTP URL to get
// hosts from or a |-separated list of host:port pairs.
func parseServerAddrSpec(spec string) (hostsEndpoint string, fallbackHosts []string) {
	if strings.HasPrefix(spec, "http") {
		return spec, nil
	}
	return "", strings.Split(spec, "|")
}

// ClientSessionState is a way to broadly track the progress of the session
type ClientSessionState uint32

const (
	Error ClientSessionState = iota
	Pristine
	Disconnected
	Connected
	Started
	Running
	Shutdown
	Unknown
)

func (s ClientSessionState) String() string {
	if s >= Unknown {
		return fmt.Sprintf("??? (%d)", s)
	}
	return [Unknown]string{
		"Error",
		"Pristine",
		"Disconnected",
		"Connected",
		"Started",
		"Running",
		"Shutdown",
	}[s]
}

type hostGetter interface {
	Get() (*gethosts.Host, error)
}

// AddresseeChecking can check if a notification can be delivered.
type AddresseeChecking interface {
	StartAddresseeBatch()
	CheckForAddressee(*protocol.Notification) *click.AppId
}

// AddressedNotification carries both a protocol.Notification and a parsed
// AppId addressee.
type AddressedNotification struct {
	To           *click.AppId
	Notification *protocol.Notification
}

// ClientSessionConfig groups the client session configuration.
type ClientSessionConfig struct {
	ConnectTimeout         time.Duration
	ExchangeTimeout        time.Duration
	HostsCachingExpiryTime time.Duration
	ExpectAllRepairedTime  time.Duration
	PEM                    []byte
	Info                   map[string]interface{}
	AuthGetter             func(string) string
	AuthURL                string
	AddresseeChecker       AddresseeChecking
	BroadcastCh            chan *BroadcastNotification
	NotificationsCh        chan AddressedNotification
}

// ClientSession holds a client<->server session and its configuration.
type ClientSession interface {
	ResetCookie()
	State() ClientSessionState
	HasConnectivity(bool)
	KeepConnection() error
	StopKeepConnection()
}

type clientSession struct {
	// configuration
	DeviceId string
	ClientSessionConfig
	SeenState    seenstate.SeenState
	Protocolator func(net.Conn) protocol.Protocol
	// hosts
	getHost                hostGetter
	fallbackHosts          []string
	deliveryHostsTimestamp time.Time
	deliveryHosts          []string
	lastAttemptTimestamp   time.Time
	leftToTry              int
	tryHost                int
	// hook for testing
	timeSince func(time.Time) time.Duration
	// connection
	connLock     sync.RWMutex
	Connection   net.Conn
	Log          logger.Logger
	TLS          *tls.Config
	proto        protocol.Protocol
	pingInterval time.Duration
	retrier      util.AutoRedialer
	cookie       string
	// status
	stateLock sync.RWMutex
	state     ClientSessionState
	// authorization
	auth string
	// autoredial knobs
	shouldDelayP    *uint32
	lastAutoRedial  time.Time
	redialDelay     func(*clientSession) time.Duration
	redialJitter    func(time.Duration) time.Duration
	redialDelays    []time.Duration
	redialDelaysIdx int
	// connection events, and cookie reset requests, come in over here
	cmdCh chan sessCmd
	// last seen connection event is here
	lastConn bool
	// connection events are handled by this
	connHandler func(bool)
	// autoredial goes over here (xxx spurious goroutine involved)
	doneCh chan uint32
	// main loop errors out through here (possibly another spurious goroutine)
	errCh chan error
	// main loop errors are handled by this
	errHandler func(error)
	// look, a stopper!
	stopCh chan struct{}
}

func redialDelay(sess *clientSession) time.Duration {
	if sess.ShouldDelay() {
		t := sess.redialDelays[sess.redialDelaysIdx]
		if len(sess.redialDelays) > sess.redialDelaysIdx+1 {
			sess.redialDelaysIdx++
		}
		return t + sess.redialJitter(t)
	} else {
		sess.redialDelaysIdx = 0
		return 0
	}
}

func NewSession(serverAddrSpec string, conf ClientSessionConfig,
	deviceId string, seenStateFactory func() (seenstate.SeenState, error),
	log logger.Logger) (*clientSession, error) {
	seenState, err := seenStateFactory()
	if err != nil {
		return nil, err
	}
	var getHost hostGetter
	log.Infof("using addr: %v", serverAddrSpec)
	hostsEndpoint, fallbackHosts := parseServerAddrSpec(serverAddrSpec)
	if hostsEndpoint != "" {
		getHost = gethosts.New(deviceId, hostsEndpoint, conf.ExchangeTimeout)
	}
	var shouldDelay uint32 = 0
	sess := &clientSession{
		ClientSessionConfig: conf,
		getHost:             getHost,
		fallbackHosts:       fallbackHosts,
		DeviceId:            deviceId,
		Log:                 log,
		Protocolator:        protocol.NewProtocol0,
		SeenState:           seenState,
		TLS:                 &tls.Config{},
		state:               Pristine,
		timeSince:           time.Since,
		shouldDelayP:        &shouldDelay,
		redialDelay:         redialDelay, // NOTE there are tests that use calling sess.redialDelay as an indication of calling autoRedial!
		redialDelays:        util.Timeouts(),
	}
	sess.redialJitter = sess.Jitter
	if sess.PEM != nil {
		cp := x509.NewCertPool()
		ok := cp.AppendCertsFromPEM(sess.PEM)
		if !ok {
			return nil, errors.New("could not parse certificate")
		}
		sess.TLS.RootCAs = cp
	}
	sess.doneCh = make(chan uint32, 1)
	sess.stopCh = make(chan struct{})
	sess.cmdCh = make(chan sessCmd)
	sess.errCh = make(chan error, 1)

	// to be overridden by tests
	sess.connHandler = sess.handleConn
	sess.errHandler = sess.handleErr

	return sess, nil
}

func (sess *clientSession) ShouldDelay() bool {
	return atomic.LoadUint32(sess.shouldDelayP) != 0
}

func (sess *clientSession) setShouldDelay() {
	atomic.StoreUint32(sess.shouldDelayP, uint32(1))
}

func (sess *clientSession) clearShouldDelay() {
	atomic.StoreUint32(sess.shouldDelayP, uint32(0))
}

func (sess *clientSession) State() ClientSessionState {
	sess.stateLock.RLock()
	defer sess.stateLock.RUnlock()
	return sess.state
}

func (sess *clientSession) setState(state ClientSessionState) {
	sess.stateLock.Lock()
	defer sess.stateLock.Unlock()
	sess.Log.Debugf("session.setState: %s -> %s", sess.state, state)
	sess.state = state
}

func (sess *clientSession) setConnection(conn net.Conn) {
	sess.connLock.Lock()
	defer sess.connLock.Unlock()
	sess.Connection = conn
}

func (sess *clientSession) getConnection() net.Conn {
	sess.connLock.RLock()
	defer sess.connLock.RUnlock()
	return sess.Connection
}

func (sess *clientSession) setCookie(cookie string) {
	sess.connLock.Lock()
	defer sess.connLock.Unlock()
	sess.cookie = cookie
}

func (sess *clientSession) getCookie() string {
	sess.connLock.RLock()
	defer sess.connLock.RUnlock()
	return sess.cookie
}

func (sess *clientSession) ResetCookie() {
	sess.cmdCh <- cmdResetCookie
}

func (sess *clientSession) resetCookie() {
	sess.stopRedial()
	sess.doClose(true)
}

// getHosts sets deliveryHosts possibly querying a remote endpoint
func (sess *clientSession) getHosts() error {
	if sess.getHost != nil {
		if sess.deliveryHosts != nil && sess.timeSince(sess.deliveryHostsTimestamp) < sess.HostsCachingExpiryTime {
			return nil
		}
		host, err := sess.getHost.Get()
		if err != nil {
			sess.Log.Errorf("getHosts: %v", err)
			sess.setState(Error)
			return err
		}
		sess.deliveryHostsTimestamp = time.Now()
		sess.deliveryHosts = host.Hosts
		if sess.TLS != nil {
			sess.TLS.ServerName = host.Domain
		}
	} else {
		sess.deliveryHosts = sess.fallbackHosts
	}
	return nil
}

// addAuthorization gets the authorization blob to send to the server
// and adds it to the session.
func (sess *clientSession) addAuthorization() error {
	if sess.AuthGetter != nil {
		sess.Log.Debugf("adding authorization")
		sess.auth = sess.AuthGetter(sess.AuthURL)
	}
	return nil
}

func (sess *clientSession) resetHosts() {
	sess.deliveryHosts = nil
}

// startConnectionAttempt/nextHostToTry help connect iterating over candidate hosts

func (sess *clientSession) startConnectionAttempt() {
	if sess.timeSince(sess.lastAttemptTimestamp) > sess.ExpectAllRepairedTime {
		sess.tryHost = 0
	}
	sess.leftToTry = len(sess.deliveryHosts)
	if sess.leftToTry == 0 {
		panic("should have got hosts from config or remote at this point")
	}
	sess.lastAttemptTimestamp = time.Now()
}

func (sess *clientSession) nextHostToTry() string {
	if sess.leftToTry == 0 {
		return ""
	}
	res := sess.deliveryHosts[sess.tryHost]
	sess.tryHost = (sess.tryHost + 1) % len(sess.deliveryHosts)
	sess.leftToTry--
	return res
}

// we reached the Started state, we can retry with the same host if we
// have to retry again
func (sess *clientSession) started() {
	sess.tryHost--
	if sess.tryHost == -1 {
		sess.tryHost = len(sess.deliveryHosts) - 1
	}
	sess.setState(Started)
}

// connect to a server using the configuration in the ClientSession
// and set up the connection.
func (sess *clientSession) connect() error {
	sess.setShouldDelay()
	sess.startConnectionAttempt()
	var err error
	var conn net.Conn
	for {
		host := sess.nextHostToTry()
		if host == "" {
			sess.setState(Error)
			return fmt.Errorf("connect: %s", err)
		}
		sess.Log.Debugf("trying to connect to: %v", host)
		conn, err = net.DialTimeout("tcp", host, sess.ConnectTimeout)
		if err == nil {
			break
		}
	}
	sess.setConnection(tls.Client(conn, sess.TLS))
	sess.setState(Connected)
	return nil
}

func (sess *clientSession) stopRedial() {
	if sess.retrier != nil {
		sess.retrier.Stop()
		sess.retrier = nil
	}
}

func (sess *clientSession) autoRedial() {
	sess.stopRedial()
	if time.Since(sess.lastAutoRedial) < 2*time.Second {
		sess.setShouldDelay()
	}
	time.Sleep(sess.redialDelay(sess))
	if sess.retrier != nil {
		panic("session AutoRedial: unexpected non-nil retrier.")
	}
	sess.retrier = util.NewAutoRedialer(sess)
	sess.lastAutoRedial = time.Now()
	go func(retrier util.AutoRedialer) {
		sess.Log.Debugf("session autoredialier launching Redial goroutine")
		// if the redialer has been stopped before calling Redial(), it'll return 0.
		sess.doneCh <- retrier.Redial()
	}(sess.retrier)
}

func (sess *clientSession) doClose(resetCookie bool) {
	sess.connLock.Lock()
	defer sess.connLock.Unlock()
	if resetCookie {
		sess.cookie = ""
	}
	sess.closeConnection()
	sess.setState(Disconnected)
}

func (sess *clientSession) closeConnection() {
	// *must be called with connLock held*
	if sess.Connection != nil {
		sess.Connection.Close()
		// we ignore Close errors, on purpose (the thinking being that
		// the connection isn't really usable, and you've got nothing
		// you could do to recover at this stage).
		sess.Connection = nil
	}
}

// handle "ping" messages
func (sess *clientSession) handlePing() error {
	err := sess.proto.WriteMessage(protocol.PingPongMsg{Type: "pong"})
	if err == nil {
		sess.Log.Debugf("ping.")
		sess.clearShouldDelay()
	} else {
		sess.setState(Error)
		sess.Log.Errorf("unable to pong: %s", err)
	}
	return err
}

func (sess *clientSession) decodeBroadcast(bcast *serverMsg) *BroadcastNotification {
	decoded := make([]map[string]interface{}, 0)
	for _, p := range bcast.Payloads {
		var v map[string]interface{}
		err := json.Unmarshal(p, &v)
		if err != nil {
			sess.Log.Debugf("expected map in broadcast: %v", err)
			continue
		}
		decoded = append(decoded, v)
	}
	return &BroadcastNotification{
		TopLevel: bcast.TopLevel,
		Decoded:  decoded,
	}
}

// handle "broadcast" messages
func (sess *clientSession) handleBroadcast(bcast *serverMsg) error {
	err := sess.SeenState.SetLevel(bcast.ChanId, bcast.TopLevel)
	if err != nil {
		sess.setState(Error)
		sess.Log.Errorf("unable to set level: %v", err)
		sess.proto.WriteMessage(protocol.AckMsg{"nak"})
		return err
	}
	// the server assumes if we ack the broadcast, we've updated
	// our levels. Hence the order.
	err = sess.proto.WriteMessage(protocol.AckMsg{"ack"})
	if err != nil {
		sess.setState(Error)
		sess.Log.Errorf("unable to ack broadcast: %s", err)
		return err
	}
	sess.clearShouldDelay()
	sess.Log.Infof("broadcast chan:%v app:%v topLevel:%d payloads:%s",
		bcast.ChanId, bcast.AppId, bcast.TopLevel, bcast.Payloads)
	if bcast.ChanId == protocol.SystemChannelId {
		// the system channel id, the only one we care about for now
		sess.Log.Debugf("sending bcast over")
		sess.BroadcastCh <- sess.decodeBroadcast(bcast)
		sess.Log.Debugf("sent bcast over")
	} else {
		sess.Log.Errorf("what is this weird channel, %#v?", bcast.ChanId)
	}
	return nil
}

// handle "notifications" messages
func (sess *clientSession) handleNotifications(ucast *serverMsg) error {
	notifs, err := sess.SeenState.FilterBySeen(ucast.Notifications)
	if err != nil {
		sess.setState(Error)
		sess.Log.Errorf("unable to record msgs seen: %v", err)
		sess.proto.WriteMessage(protocol.AckMsg{"nak"})
		return err
	}
	// the server assumes if we ack the broadcast, we've updated
	// our state. Hence the order.
	err = sess.proto.WriteMessage(protocol.AckMsg{"ack"})
	if err != nil {
		sess.setState(Error)
		sess.Log.Errorf("unable to ack notifications: %s", err)
		return err
	}
	sess.clearShouldDelay()
	sess.AddresseeChecker.StartAddresseeBatch()
	for i := range notifs {
		notif := &notifs[i]
		to := sess.AddresseeChecker.CheckForAddressee(notif)
		if to == nil {
			continue
		}
		sess.Log.Infof("unicast app:%v msg:%s payload:%s",
			notif.AppId, notif.MsgId, notif.Payload)
		sess.Log.Debugf("sending ucast over")
		sess.NotificationsCh <- AddressedNotification{to, notif}
		sess.Log.Debugf("sent ucast over")
	}
	return nil
}

// handle "connbroken" messages
func (sess *clientSession) handleConnBroken(connBroken *serverMsg) error {
	sess.setState(Error)
	reason := connBroken.Reason
	err := fmt.Errorf("server broke connection: %s", reason)
	sess.Log.Errorf("%s", err)
	switch reason {
	case protocol.BrokenHostMismatch:
		sess.resetHosts()
	}
	return err
}

// handle "setparams" messages
func (sess *clientSession) handleSetParams(setParams *serverMsg) error {
	if setParams.SetCookie != "" {
		sess.setCookie(setParams.SetCookie)
	}
	return nil
}

// loop runs the session with the server, emits a stream of events.
func (sess *clientSession) loop() error {
	var err error
	var recv serverMsg
	sess.setState(Running)
	for {
		deadAfter := sess.pingInterval + sess.ExchangeTimeout
		sess.proto.SetDeadline(time.Now().Add(deadAfter))
		err = sess.proto.ReadMessage(&recv)
		if err != nil {
			sess.Log.Debugf("session aborting with error on read.")
			sess.setState(Error)
			return err
		}
		switch recv.Type {
		case "ping":
			err = sess.handlePing()
		case "broadcast":
			err = sess.handleBroadcast(&recv)
		case "notifications":
			err = sess.handleNotifications(&recv)
		case "connbroken":
			err = sess.handleConnBroken(&recv)
		case "setparams":
			err = sess.handleSetParams(&recv)
		case "warn":
			// XXX: current message "warn" should be "connwarn"
			fallthrough
		case "connwarn":
			sess.Log.Errorf("server sent warning: %s", recv.Reason)
		}
		if err != nil {
			sess.Log.Debugf("session aborting with error from handler.")
			return err
		}
	}
}

// Call this when you've connected and want to start looping.
func (sess *clientSession) start() error {
	conn := sess.getConnection()
	err := conn.SetDeadline(time.Now().Add(sess.ExchangeTimeout))
	if err != nil {
		sess.setState(Error)
		sess.Log.Errorf("unable to start: set deadline: %s", err)
		return err
	}
	_, err = conn.Write(wireVersionBytes)
	// The Writer docs: Write must return a non-nil error if it returns
	// n < len(p). So, no need to check number of bytes written, hooray.
	if err != nil {
		sess.setState(Error)
		sess.Log.Errorf("unable to start: write version: %s", err)
		return err
	}
	proto := sess.Protocolator(conn)
	proto.SetDeadline(time.Now().Add(sess.ExchangeTimeout))
	levels, err := sess.SeenState.GetAllLevels()
	if err != nil {
		sess.setState(Error)
		sess.Log.Errorf("unable to start: get levels: %v", err)
		return err
	}
	err = proto.WriteMessage(protocol.ConnectMsg{
		Type:          "connect",
		DeviceId:      sess.DeviceId,
		Authorization: sess.auth,
		Cookie:        sess.getCookie(),
		Levels:        levels,
		Info:          sess.Info,
	})
	if err != nil {
		sess.setState(Error)
		sess.Log.Errorf("unable to start: connect: %s", err)
		return err
	}
	var connAck protocol.ConnAckMsg
	err = proto.ReadMessage(&connAck)
	if err != nil {
		sess.setState(Error)
		sess.Log.Errorf("unable to start: connack: %s", err)
		return err
	}
	if connAck.Type != "connack" {
		sess.setState(Error)
		return fmt.Errorf("expecting CONNACK, got %#v", connAck.Type)
	}
	pingInterval, err := time.ParseDuration(connAck.Params.PingInterval)
	if err != nil {
		sess.setState(Error)
		sess.Log.Errorf("unable to start: parse ping interval: %s", err)
		return err
	}
	sess.proto = proto
	sess.pingInterval = pingInterval
	sess.Log.Debugf("connected %v.", conn.RemoteAddr())
	sess.started() // deals with choosing which host to retry with as well
	return nil
}

// run calls connect, and if it works it calls start, and if it works
// it runs loop in a goroutine, and ships its return value over ErrCh.
func (sess *clientSession) run(closer func(bool), authChecker, hostGetter, connecter, starter, looper func() error) error {
	closer(false)
	if err := authChecker(); err != nil {
		return err
	}
	if err := hostGetter(); err != nil {
		return err
	}
	err := connecter()
	if err == nil {
		err = starter()
		if err == nil {
			go func() { sess.errCh <- looper() }()
		}
	}
	return err
}

// This Jitter returns a random time.Duration somewhere in [-spread, spread].
func (sess *clientSession) Jitter(spread time.Duration) time.Duration {
	if spread < 0 {
		panic("spread must be non-negative")
	}
	n := int64(spread)
	return time.Duration(rand.Int63n(2*n+1) - n)
}

// Dial takes the session from newly created (or newly disconnected)
// to running the main loop.
func (sess *clientSession) Dial() error {
	if sess.Protocolator == nil {
		// a missing protocolator means you've willfully overridden
		// it; returning an error here would prompt AutoRedial to just
		// keep on trying.
		panic("can't Dial() without a protocol constructor.")
	}
	return sess.run(sess.doClose, sess.addAuthorization, sess.getHosts, sess.connect, sess.start, sess.loop)
}

func (sess *clientSession) doKeepConnection() {
Loop:
	for {
		select {
		case cmd := <-sess.cmdCh:
			switch cmd {
			case cmdConnect:
				sess.connHandler(true)
			case cmdDisconnect:
				sess.connHandler(false)
			case cmdResetCookie:
				sess.resetCookie()
			}
		case <-sess.stopCh:
			sess.Log.Infof("session shutting down.")
			sess.connLock.Lock()
			defer sess.connLock.Unlock()
			sess.stopRedial()
			sess.closeConnection()
			break Loop
		case n := <-sess.doneCh:
			// if n == 0, the redialer aborted. If you do
			// anything other than log it, keep that in mind.
			sess.Log.Debugf("connected after %d attempts.", n)
		case err := <-sess.errCh:
			sess.errHandler(err)
		}
	}
}

func (sess *clientSession) handleConn(hasConn bool) {
	sess.lastConn = hasConn

	// Note this does not depend on the current state!  That's because Dial
	// starts with doClose, which gets you to Disconnected even if you're
	// connected, and you can call Close when Disconnected without it
	// losing its stuff.
	if hasConn {
		sess.autoRedial()
	} else {
		sess.stopRedial()
		sess.doClose(false)
	}
}

func (sess *clientSession) handleErr(err error) {
	sess.Log.Errorf("session error'ed out with %v", err)
	sess.stateLock.Lock()
	if sess.state == Disconnected && sess.lastConn {
		sess.autoRedial()
	}
	sess.stateLock.Unlock()
}

func (sess *clientSession) KeepConnection() error {
	sess.stateLock.Lock()
	defer sess.stateLock.Unlock()
	if sess.state != Pristine {
		return errors.New("don't call KeepConnection() on a non-pristine session.")
	}
	sess.state = Disconnected

	go sess.doKeepConnection()

	return nil
}

func (sess *clientSession) StopKeepConnection() {
	sess.setState(Shutdown)
	close(sess.stopCh)
}

func (sess *clientSession) HasConnectivity(hasConn bool) {
	if hasConn {
		sess.cmdCh <- cmdConnect
	} else {
		sess.cmdCh <- cmdDisconnect
	}
}

func init() {
	rand.Seed(time.Now().Unix()) // good enough for us (we're not using it for crypto)
}
