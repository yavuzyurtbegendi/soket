package soket

import (
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/soket/config"
)

type IHaus interface {
	filterSessions(func(*Session) bool) map[*Session]struct{}
	filterSessionsByTag(string) map[*Session]struct{}
	getAllSessions() map[*Session]struct{}

	registerSession(*Session, map[string]struct{})
	unregisterSession(*Session)
	broadcastTo(map[*Session]struct{}, *packet)

	isOpen() bool
	close()
}

type haus struct {
	sessions      map[*Session]struct{}
	sessionsMutex *sync.RWMutex

	sessionsWithTags      map[string]map[*Session]struct{}
	sessionsWithTagsMutex *sync.RWMutex

	handlers *handlers

	conf *config.Config

	state int32
}

func newHaus(conf *config.Config, handlers *handlers) IHaus {
	return &haus{
		sessions:              make(map[*Session]struct{}),
		sessionsMutex:         &sync.RWMutex{},
		sessionsWithTags:      make(map[string]map[*Session]struct{}),
		sessionsWithTagsMutex: &sync.RWMutex{},
		handlers:              handlers,
		conf:                  conf,
		state:                 OPENED,
	}
}

func (h *haus) filterSessionsByTag(tag string) map[*Session]struct{} {
	h.sessionsWithTagsMutex.RLock()
	defer h.sessionsWithTagsMutex.RUnlock()
	return h.sessionsWithTags[tag]
}

func (h *haus) filterSessions(filter func(*Session) bool) map[*Session]struct{} {
	sessions := make(map[*Session]struct{})
	h.sessionsMutex.RLock()
	for session := range h.sessions {
		if filter(session) {
			sessions[session] = struct{}{}
		}
	}
	h.sessionsMutex.RUnlock()
	return sessions
}

func (h *haus) getAllSessions() map[*Session]struct{} {
	h.sessionsMutex.RLock()
	defer h.sessionsMutex.RUnlock()
	return h.sessions
}

func (h *haus) registerSession(session *Session, tags map[string]struct{}) {
	h.sessionsWithTagsMutex.Lock()
	for tag := range tags {
		_, ok := h.sessionsWithTags[tag]
		if !ok {
			h.sessionsWithTags[tag] = make(map[*Session]struct{})
		}
		h.sessionsWithTags[tag][session] = struct{}{}
		if session.tags == nil {
			session.tags = make(map[string]struct{})
		}
		session.tags[tag] = struct{}{}
	}
	h.sessionsWithTagsMutex.Unlock()

	h.sessionsMutex.Lock()
	h.sessions[session] = struct{}{}
	h.sessionsMutex.Unlock()

	h.handlers.logHandler(session, "SESSION_REGISTERED")
}

// you need to write tests for this one
func (h *haus) unregisterSession(session *Session) {
	var tags map[string]struct{}
	h.sessionsMutex.Lock()
	tags = session.tags
	delete(h.sessions, session)
	h.sessionsMutex.Unlock()

	h.sessionsWithTagsMutex.Lock()
	for tag := range tags {
		delete(h.sessionsWithTags[tag], session)
	}
	h.sessionsWithTagsMutex.Unlock()

	h.handlers.logHandler(session, "SESSION_UNREGISTERED")
}

func (h *haus) broadcastTo(sessions map[*Session]struct{}, pck *packet) {
	if !h.isOpen() && pck.eType != websocket.CloseMessage {
		return
	}
	for s := range sessions {
		if s.closed {
			h.handlers.logHandler(s, "CANNOT_SEND_TO_CLOSED_SESSION")
			continue
		}
		s.writeMessageToPipe(pck)
	}
}

const (
	// OPENED represents that the haus is opened.
	OPENED = iota

	// CLOSED represents that the haus is closed.
	CLOSED
)

func (h *haus) isOpen() bool {
	return atomic.LoadInt32(&h.state) == OPENED
}

func (h *haus) close() {
	atomic.StoreInt32(&h.state, CLOSED)
}
