package main

import (
	"net"

	smux "gopkg.in/xtaci/smux.v1"
)

type muxStreamConn struct {
	net.Conn
	stream *smux.Stream
}

func (c *muxStreamConn) Read(b []byte) (n int, err error) {
	return c.stream.Read(b)
}

func (c *muxStreamConn) Write(b []byte) (n int, err error) {
	return c.stream.Write(b)
}

func (c *muxStreamConn) Close() error {
	return c.stream.Close()
}

type muxSession struct {
	id      uint16
	conn    net.Conn
	session *smux.Session
}

func (session *muxSession) GetConn() (net.Conn, error) {
	stream, err := session.session.OpenStream()
	if err != nil {
		return nil, err
	}
	return &muxStreamConn{Conn: session.conn, stream: stream}, nil
}

func (session *muxSession) Accept() (net.Conn, error) {
	stream, err := session.session.AcceptStream()
	if err != nil {
		return nil, err
	}
	return &muxStreamConn{Conn: session.conn, stream: stream}, nil
}

func (session *muxSession) Close() error {
	if session.session == nil {
		return nil
	}
	return session.session.Close()
}

func (session *muxSession) IsClosed() bool {
	if session.session == nil {
		return true
	}
	return session.session.IsClosed()
}

func (session *muxSession) NumStreams() int {
	return session.session.NumStreams()
}

type sessionManager struct {
	sessions       []*muxSession
	maxSessions    uint16
	currentSession uint16
}

func SessionManager(maxSessions uint16) *sessionManager {
	return &sessionManager{maxSessions: maxSessions,
		sessions: make([]*muxSession, maxSessions)}
}

func (sm *sessionManager) Get() *muxSession {
	session := sm.sessions[sm.currentSession]
	if session == nil {
		return nil
	}

	if session.session != nil && session.session.IsClosed() {
		sm.sessions[sm.currentSession] = nil
		return nil
	}

	sm.Next()
	return session
}

func (sm *sessionManager) Allocate(conn net.Conn) *muxSession {
	mSession := &muxSession{
		id:   sm.currentSession,
		conn: conn,
	}

	sm.sessions[sm.currentSession] = mSession
	sm.Next()
	return mSession
}

func (sm *sessionManager) Remove(session *muxSession) {
	sm.sessions[session.id] = nil
}

func (sm *sessionManager) Size() int {
	return len(sm.sessions)
}

func (sm *sessionManager) Next() {
	sm.currentSession = (sm.currentSession + 1) % sm.maxSessions
}

type muxConn struct {
	net.Conn
	session *muxSession
}
