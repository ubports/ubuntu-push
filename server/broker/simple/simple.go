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

	"launchpad.net/ubuntu-push/logger"
	"launchpad.net/ubuntu-push/protocol"
	"launchpad.net/ubuntu-push/server/broker"
	"launchpad.net/ubuntu-push/server/store"
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
}

// simpleBrokerSession represents a session in the broker.
type simpleBrokerSession struct {
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

// NewSimpleBroker makes a new SimpleBroker.
func NewSimpleBroker(sto store.PendingStore, cfg broker.BrokerConfig, logger logger.Logger) *SimpleBroker {
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

func (b *SimpleBroker) feedPending(sess *simpleBrokerSession) error {
	// find relevant channels, for now only system
	channels := []store.InternalChannelId{store.SystemInternalChannelId}
	for _, chanId := range channels {
		topLevel, payloads, err := b.sto.GetChannelSnapshot(chanId)
		if err != nil {
			// next broadcast will try again
			b.logger.Errorf("unsuccessful feed pending, get channel snapshot for %v: %v", chanId, err)
			continue
		}
		clientLevel := sess.levels[chanId]
		if clientLevel != topLevel {
			broadcastExchg := &broker.BroadcastExchange{
				ChanId:               chanId,
				TopLevel:             topLevel,
				NotificationPayloads: payloads,
			}
			broadcastExchg.Init()
			sess.exchanges <- broadcastExchg
		}
	}
	return nil
}

// Register registers a session with the broker. It feeds the session
// pending notifications as well.
func (b *SimpleBroker) Register(connect *protocol.ConnectMsg) (broker.BrokerSession, error) {
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
		deviceId:     connect.DeviceId,
		model:        model,
		imageChannel: imageChannel,
		done:         make(chan bool),
		exchanges:    make(chan broker.Exchange, b.sessionQueueSize),
		levels:       levels,
	}
	b.sessionCh <- sess
	<-sess.done
	err = b.feedPending(sess)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

// Unregister unregisters a session with the broker. Doesn't wait.
func (b *SimpleBroker) Unregister(s broker.BrokerSession) {
	sess := s.(*simpleBrokerSession)
	b.sessionCh <- sess
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
				b.registry[sess.deviceId] = sess
				sess.registered = true
				sess.done <- true
			}
		case delivery := <-b.deliveryCh:
			switch delivery.kind {
			case broadcastDelivery:
				topLevel, payloads, err := b.sto.GetChannelSnapshot(delivery.chanId)
				if err != nil {
					// next broadcast will try again
					b.logger.Errorf("unsuccessful broadcast, get channel snapshot for %v: %v", delivery.chanId, err)
					continue Loop
				}
				broadcastExchg := &broker.BroadcastExchange{
					ChanId:               delivery.chanId,
					TopLevel:             topLevel,
					NotificationPayloads: payloads,
				}
				broadcastExchg.Init()
				for _, sess := range b.registry {
					sess.exchanges <- broadcastExchg
				}
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
