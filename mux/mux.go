package mux

import (
	"context"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/log"
	"github.com/xtaci/smux"
)

type muxID uint32

func generateMuxID() muxID {
	return muxID(rand.Uint32())
}

type muxSession struct {
	id             muxID
	client         client
	underlayConn   net.Conn
	lastActiveTime time.Time
}

type MuxPool struct {
	sync.Mutex
	concurrency uint
	timeout     time.Duration
	ctx         context.Context
	sessions    map[muxID]*muxSession
}

func (p *MuxPool) DialMux(newConn func() (net.Conn, error)) (net.Conn, error) {
	openNewStream := func(sess *muxSession) (net.Conn, error) {
		rwc, err := sess.client.OpenNewStream()
		sess.lastActiveTime = time.Now()
		if err != nil {
			sess.underlayConn.Close()
			sess.client.Close()
			delete(p.sessions, sess.id)
			return nil, err
		}

		return rwc, err
	}

	p.Lock()
	defer p.Unlock()
	for _, sess := range p.sessions {
		if sess.client.IsClosed() {
			delete(p.sessions, sess.id)
			continue
		}

		if sess.client.NumStreams() < int(p.concurrency) || p.concurrency <= 0 {
			return openNewStream(sess)
		}
	}

	id := generateMuxID()
	conn, err := newConn()
	if err != nil {
		return nil, err
	}

	smuxConfig := smux.DefaultConfig()
	client, err := smux.Client(conn, smuxConfig)
	if err != nil {
		return nil, err
	}

	sess := &muxSession{id: id, client: &smuxClient{client}, underlayConn: conn}
	p.sessions[id] = sess
	return openNewStream(sess)
}

func (p *MuxPool) cleanLoop() {
	var checkDuration time.Duration
	checkDuration = p.timeout / 4

	for {
		select {
		case <-time.After(checkDuration):
			p.Lock()
			for id, sess := range p.sessions {
				if sess.client.IsClosed() {
					sess.client.Close()
					sess.underlayConn.Close()
					delete(p.sessions, id)
				} else if sess.client.NumStreams() == 0 && time.Now().Sub(sess.lastActiveTime) > p.timeout {
					sess.client.Close()
					sess.underlayConn.Close()
					delete(p.sessions, id)
				}
			}
			p.Unlock()
		case <-p.ctx.Done():
			p.Lock()
			for id, sess := range p.sessions {
				sess.client.Close()
				sess.underlayConn.Close()
				delete(p.sessions, id)
			}
			p.Unlock()

			log.Infoln("all mux sessions are closed")
			return
		}
	}
}

func NewMuxPool(ctx context.Context) *MuxPool {
	options := ctx.Value(C.OPTIONS).(*args.Options)
	pool := &MuxPool{
		ctx:         ctx,
		concurrency: options.Mux,
		timeout:     time.Duration(30) * time.Second,
		sessions:    make(map[muxID]*muxSession),
	}

	go pool.cleanLoop()
	return pool
}
