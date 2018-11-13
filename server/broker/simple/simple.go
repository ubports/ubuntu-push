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

// Package simple implements a simple broker for just one process.
package simple

import (
	"sync"

	"github.com/ubports/ubuntu-push/logger"
	"github.com/ubports/ubuntu-push/protocol"
	"github.com/ubports/ubuntu-push/server/broker"
	"github.com/ubports/ubuntu-push/server/store"
	"github.com/ubports/ubuntu-push/server/statistics"
)

// SimpleBroker implements broker.Broker/BrokerSending for everything
// in just one process.
type SimpleBroker struct {
	sto    store.PendingStore
	logger logger.Logger
	// running state
	runMutex sync.Mutex
	running  bool
	stop     chan bool
	stopped  chan bool
	// sessions
	sessionCh        chan *simpleBrokerSession
	registry         map[string]*simpleBrokerSession
	sessionQueueSize uint
	// delivery
	deliveryCh chan *delivery
	currentStats *statistics.Statistics
}

// simpleBrokerSession represents a session in the broker.
type simpleBrokerSession struct {
	broker       *SimpleBroker
	registered   bool
	deviceId     string
	model        string
	imageChannel string
	done         chan bool
	exchanges    chan broker.Exchange
	levels       broker.LevelsMap
	// for exchanges
	exchgScratch broker.ExchangesScratchArea
}

type deliveryKind int

const (
	broadcastDelivery deliveryKind = iota
	unicastDelivery
)

// delivery holds all the information to request a delivery
type delivery struct {
	kind   deliveryKind
	chanId store.InternalChannelId
}

func (sess *simpleBrokerSession) SessionChannel() <-chan broker.Exchange {
	return sess.exchanges
}

func (sess *simpleBrokerSession) DeviceIdentifier() string {
	return sess.deviceId
}

func (sess *simpleBrokerSession) DeviceImageModel() string {
	return sess.model
}

func (sess *simpleBrokerSession) DeviceImageChannel() string {
	return sess.imageChannel
}

func (sess *simpleBrokerSession) Levels() broker.LevelsMap {
	return sess.levels
}

func (sess *simpleBrokerSession) ExchangeScratchArea() *broker.ExchangesScratchArea {
	return &sess.exchgScratch
}

func (sess *simpleBrokerSession) Get(chanId store.InternalChannelId, cachedOk bool) (int64, []protocol.Notification, error) {
	return sess.broker.get(chanId, cachedOk)
}

func (sess *simpleBrokerSession) DropByMsgId(chanId store.InternalChannelId, targets []protocol.Notification) error {
	return sess.broker.drop(chanId, targets)
}

func (sess *simpleBrokerSession) Feed(exchg broker.Exchange) {
	sess.exchanges <- exchg
}

func (sess *simpleBrokerSession) InternalChannelId() store.InternalChannelId {
	return store.UnicastInternalChannelId(sess.deviceId, sess.deviceId)
}

// NewSimpleBroker makes a new SimpleBroker.
func NewSimpleBroker(sto store.PendingStore, cfg broker.BrokerConfig, logger logger.Logger, currentStats *statistics.Statistics) *SimpleBroker {
	sessionCh := make(chan *simpleBrokerSession, cfg.BrokerQueueSize())
	deliveryCh := make(chan *delivery, cfg.BrokerQueueSize())
	registry := make(map[string]*simpleBrokerSession)
	return &SimpleBroker{
		logger:           logger,
		sto:              sto,
		stop:             make(chan bool),
		stopped:          make(chan bool),
		registry:         registry,
		sessionCh:        sessionCh,
		deliveryCh:       deliveryCh,
		sessionQueueSize: cfg.SessionQueueSize(),
		currentStats:	currentStats,
	}
}

// Start starts the broker.
func (b *SimpleBroker) Start() {
	b.runMutex.Lock()
	defer b.runMutex.Unlock()
	if b.running {
		return
	}
	b.running = true
	go b.run()
}

// Stop stops the broker.
func (b *SimpleBroker) Stop() {
	b.runMutex.Lock()
	defer b.runMutex.Unlock()
	if !b.running {
		return
	}
	b.stop <- true
	<-b.stopped
	b.running = false
}

// Running returns whether ther broker is running.
func (b *SimpleBroker) Running() bool {
	b.runMutex.Lock()
	defer b.runMutex.Unlock()
	return b.running
}

// Register registers a session with the broker. It feeds the session
// pending notifications as well.
func (b *SimpleBroker) Register(connect *protocol.ConnectMsg, track broker.SessionTracker) (broker.BrokerSession, error) {
	// xxx sanity check DeviceId
	model, err := broker.GetInfoString(connect, "device", "?")
	if err != nil {
		return nil, err
	}
	imageChannel, err := broker.GetInfoString(connect, "channel", "?")
	if err != nil {
		return nil, err
	}
	levels := map[store.InternalChannelId]int64{}
	for hexId, v := range connect.Levels {
		id, err := store.HexToInternalChannelId(hexId)
		if err != nil {
			return nil, &broker.ErrAbort{err.Error()}
		}
		levels[id] = v
	}
	sess := &simpleBrokerSession{
		broker:       b,
		deviceId:     connect.DeviceId,
		model:        model,
		imageChannel: imageChannel,
		done:         make(chan bool),
		exchanges:    make(chan broker.Exchange, b.sessionQueueSize),
		levels:       levels,
	}
	b.sessionCh <- sess
	<-sess.done
	err = broker.FeedPending(sess)
	if err != nil {
		return nil, err
	}
	b.logger.Infof("Registered the following device info: %v %v", sess.model, sess.imageChannel)
	b.currentStats.IncreaseDevices(sess.model, sess.imageChannel)
	return sess, nil
}

// Unregister unregisters a session with the broker. Doesn't wait.
func (b *SimpleBroker) Unregister(s broker.BrokerSession) {
	sess := s.(*simpleBrokerSession)
	b.sessionCh <- sess
	b.currentStats.DecreaseDevices(sess.model, sess.imageChannel)
}

func (b *SimpleBroker) get(chanId store.InternalChannelId, cachedOk bool) (int64, []protocol.Notification, error) {
	topLevel, notifications, err := b.sto.GetChannelSnapshot(chanId)
	if err != nil {
		b.logger.Errorf("unsuccessful, get channel snapshot for %v (cachedOk=%v): %v", chanId, cachedOk, err)
	}
	return topLevel, notifications, err

}

func (b *SimpleBroker) drop(chanId store.InternalChannelId, targets []protocol.Notification) error {
	err := b.sto.DropByMsgId(chanId, targets)
	if err != nil {
		b.logger.Errorf("unsuccessful, drop from channel %v: %v", chanId, err)
	}
	return err

}

// run runs the agent logic of the broker.
func (b *SimpleBroker) run() {
Loop:
	for {
		select {
		case <-b.stop:
			b.stopped <- true
			break Loop
		case sess := <-b.sessionCh:
			if sess.registered { // unregister
				// unregister only current
				if b.registry[sess.deviceId] == sess {
					delete(b.registry, sess.deviceId)
				}
			} else { // register
				prev := b.registry[sess.deviceId]
				if prev != nil { // kick it
					prev.exchanges <- nil
				}
				b.registry[sess.deviceId] = sess
				sess.registered = true
				sess.done <- true
			}
		case delivery := <-b.deliveryCh:
			switch delivery.kind {
			case broadcastDelivery:
				topLevel, notifications, err := b.get(delivery.chanId, false)
				if err != nil {
					// next broadcast will try again
					continue Loop
				}
				broadcastExchg := &broker.BroadcastExchange{
					ChanId:        delivery.chanId,
					TopLevel:      topLevel,
					Notifications: notifications,
				}
				broadcastExchg.Init()
				for _, sess := range b.registry {
					sess.exchanges <- broadcastExchg
				}
				b.currentStats.IncreaseBroadcasts()
			case unicastDelivery:
				chanId := delivery.chanId
				_, devId := chanId.UnicastUserAndDevice()
				sess := b.registry[devId]
				if sess != nil {
					sess.exchanges <- &broker.UnicastExchange{ChanId: chanId, CachedOk: false}
				}
				b.currentStats.IncreaseUnicasts()
			}
		}
	}
}

// Broadcast requests the broadcast for a channel.
func (b *SimpleBroker) Broadcast(chanId store.InternalChannelId) {
	b.deliveryCh <- &delivery{
		kind:   broadcastDelivery,
		chanId: chanId,
	}
}

// Unicast requests unicast for the channels.
func (b *SimpleBroker) Unicast(chanIds ...store.InternalChannelId) {
	for _, chanId := range chanIds {
		b.deliveryCh <- &delivery{
			kind:   unicastDelivery,
			chanId: chanId,
		}
	}
}
