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

	"launchpad.net/ubuntu-push/client/gethosts"
	"launchpad.net/ubuntu-push/client/session/seenstate"
	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/util"
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
	Disconnected
	Connected
	Started
	Running
)

type hostGetter interface {
	Get() (*gethosts.Host, error)
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
}

// ClientSession holds a client<->server session and its configuration.
type ClientSession struct {
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
	// status
	stateP          *uint32
	ErrCh           chan error
	BroadcastCh     chan *BroadcastNotification
	NotificationsCh chan *protocol.Notification
	// authorization
	auth string
	// autoredial knobs
	shouldDelayP    *uint32
	lastAutoRedial  time.Time
	redialDelay     func(*ClientSession) time.Duration
	redialJitter    func(time.Duration) time.Duration
	redialDelays    []time.Duration
	redialDelaysIdx int
}

func redialDelay(sess *ClientSession) time.Duration {
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
	log logger.Logger) (*ClientSession, error) {
	state := uint32(Disconnected)
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
	sess := &ClientSession{
		ClientSessionConfig: conf,
		getHost:             getHost,
		fallbackHosts:       fallbackHosts,
		DeviceId:            deviceId,
		Log:                 log,
		Protocolator:        protocol.NewProtocol0,
		SeenState:           seenState,
		TLS:                 &tls.Config{},
		stateP:              &state,
		timeSince:           time.Since,
		shouldDelayP:        &shouldDelay,
		redialDelay:         redialDelay,
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
	return sess, nil
}

func (sess *ClientSession) ShouldDelay() bool {
	return atomic.LoadUint32(sess.shouldDelayP) != 0
}

func (sess *ClientSession) setShouldDelay() {
	atomic.StoreUint32(sess.shouldDelayP, uint32(1))
}

func (sess *ClientSession) clearShouldDelay() {
	atomic.StoreUint32(sess.shouldDelayP, uint32(0))
}

func (sess *ClientSession) State() ClientSessionState {
	return ClientSessionState(atomic.LoadUint32(sess.stateP))
}

func (sess *ClientSession) setState(state ClientSessionState) {
	atomic.StoreUint32(sess.stateP, uint32(state))
}

func (sess *ClientSession) setConnection(conn net.Conn) {
	sess.connLock.Lock()
	defer sess.connLock.Unlock()
	sess.Connection = conn
}

func (sess *ClientSession) getConnection() net.Conn {
	sess.connLock.RLock()
	defer sess.connLock.RUnlock()
	return sess.Connection
}

// getHosts sets deliveryHosts possibly querying a remote endpoint
func (sess *ClientSession) getHosts() error {
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
func (sess *ClientSession) addAuthorization() error {
	if sess.AuthGetter != nil {
		sess.Log.Debugf("adding authorization")
		sess.auth = sess.AuthGetter(sess.AuthURL)
	}
	return nil
}

func (sess *ClientSession) resetHosts() {
	sess.deliveryHosts = nil
}

// startConnectionAttempt/nextHostToTry help connect iterating over candidate hosts

func (sess *ClientSession) startConnectionAttempt() {
	if sess.timeSince(sess.lastAttemptTimestamp) > sess.ExpectAllRepairedTime {
		sess.tryHost = 0
	}
	sess.leftToTry = len(sess.deliveryHosts)
	if sess.leftToTry == 0 {
		panic("should have got hosts from config or remote at this point")
	}
	sess.lastAttemptTimestamp = time.Now()
}

func (sess *ClientSession) nextHostToTry() string {
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
func (sess *ClientSession) started() {
	sess.tryHost--
	if sess.tryHost == -1 {
		sess.tryHost = len(sess.deliveryHosts) - 1
	}
	sess.setState(Started)
}

// connect to a server using the configuration in the ClientSession
// and set up the connection.
func (sess *ClientSession) connect() error {
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

func (sess *ClientSession) stopRedial() {
	if sess.retrier != nil {
		sess.retrier.Stop()
		sess.retrier = nil
	}
}

func (sess *ClientSession) AutoRedial(doneCh chan uint32) {
	sess.stopRedial()
	if time.Since(sess.lastAutoRedial) < 2*time.Second {
		sess.setShouldDelay()
	}
	time.Sleep(sess.redialDelay(sess))
	sess.retrier = util.NewAutoRedialer(sess)
	sess.lastAutoRedial = time.Now()
	go func() { doneCh <- sess.retrier.Redial() }()
}

func (sess *ClientSession) Close() {
	sess.stopRedial()
	sess.doClose()
}
func (sess *ClientSession) doClose() {
	sess.connLock.Lock()
	defer sess.connLock.Unlock()
	if sess.Connection != nil {
		sess.Connection.Close()
		// we ignore Close errors, on purpose (the thinking being that
		// the connection isn't really usable, and you've got nothing
		// you could do to recover at this stage).
		sess.Connection = nil
	}
	sess.setState(Disconnected)
}

// handle "ping" messages
func (sess *ClientSession) handlePing() error {
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

func (sess *ClientSession) decodeBroadcast(bcast *serverMsg) *BroadcastNotification {
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
func (sess *ClientSession) handleBroadcast(bcast *serverMsg) error {
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
	sess.Log.Debugf("broadcast chan:%v app:%v topLevel:%d payloads:%s",
		bcast.ChanId, bcast.AppId, bcast.TopLevel, bcast.Payloads)
	if bcast.ChanId == protocol.SystemChannelId {
		// the system channel id, the only one we care about for now
		sess.Log.Debugf("sending bcast over")
		sess.BroadcastCh <- sess.decodeBroadcast(bcast)
		sess.Log.Debugf("sent bcast over")
	} else {
		sess.Log.Debugf("what is this weird channel, %#v?", bcast.ChanId)
	}
	return nil
}

// handle "notifications" messages
func (sess *ClientSession) handleNotifications(ucast *serverMsg) error {
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
	for i := range notifs {
		notif := &notifs[i]
		sess.Log.Debugf("unicast app:%v msg:%s payload:%s",
			notif.AppId, notif.MsgId, notif.Payload)
		sess.Log.Debugf("sending ucast over")
		sess.NotificationsCh <- notif
		sess.Log.Debugf("sent ucast over")
	}
	return nil
}

// handle "connbroken" messages
func (sess *ClientSession) handleConnBroken(connBroken *serverMsg) error {
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

// loop runs the session with the server, emits a stream of events.
func (sess *ClientSession) loop() error {
	var err error
	var recv serverMsg
	sess.setState(Running)
	for {
		deadAfter := sess.pingInterval + sess.ExchangeTimeout
		sess.proto.SetDeadline(time.Now().Add(deadAfter))
		err = sess.proto.ReadMessage(&recv)
		if err != nil {
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
		case "warn":
			// XXX: current message "warn" should be "connwarn"
			fallthrough
		case "connwarn":
			sess.Log.Errorf("server sent warning: %s", recv.Reason)
		}
		if err != nil {
			return err
		}
	}
}

// Call this when you've connected and want to start looping.
func (sess *ClientSession) start() error {
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
	sess.Log.Debugf("Connected %v.", conn.RemoteAddr())
	sess.started() // deals with choosing which host to retry with as well
	return nil
}

// run calls connect, and if it works it calls start, and if it works
// it runs loop in a goroutine, and ships its return value over ErrCh.
func (sess *ClientSession) run(closer func(), authChecker, hostGetter, connecter, starter, looper func() error) error {
	closer()
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
			sess.ErrCh = make(chan error, 1)
			sess.BroadcastCh = make(chan *BroadcastNotification)
			sess.NotificationsCh = make(chan *protocol.Notification)
			go func() { sess.ErrCh <- looper() }()
		}
	}
	return err
}

// This Jitter returns a random time.Duration somewhere in [-spread, spread].
func (sess *ClientSession) Jitter(spread time.Duration) time.Duration {
	if spread < 0 {
		panic("spread must be non-negative")
	}
	n := int64(spread)
	return time.Duration(rand.Int63n(2*n+1) - n)
}

// Dial takes the session from newly created (or newly disconnected)
// to running the main loop.
func (sess *ClientSession) Dial() error {
	if sess.Protocolator == nil {
		// a missing protocolator means you've willfully overridden
		// it; returning an error here would prompt AutoRedial to just
		// keep on trying.
		panic("can't Dial() without a protocol constructor.")
	}
	return sess.run(sess.doClose, sess.addAuthorization, sess.getHosts, sess.connect, sess.start, sess.loop)
}

func init() {
	rand.Seed(time.Now().Unix()) // good enough for us (we're not using it for crypto)
}
