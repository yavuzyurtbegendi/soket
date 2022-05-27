package soket

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterSessionsByTag(t *testing.T) {
	h := &haus{
		sessionsWithTagsMutex: &sync.RWMutex{},
		sessionsWithTags: map[string]map[*Session]struct{}{
			"tag-exists": {
				{}: {},
			},
		},
	}
	sessions := h.filterSessionsByTag("tag-exists")
	assert.Len(t, sessions, 1)

	sessions = h.filterSessionsByTag("tag-not-exists")
	assert.Len(t, sessions, 0)
}

func TestFilterSessions(t *testing.T) {
	h := &haus{
		sessionsMutex: &sync.RWMutex{},
		sessions: map[*Session]struct{}{
			{
				id: "id1",
			}: {},
			{
				id: "id2",
			}: {},
		},
	}
	sessions := h.filterSessions(func(s *Session) bool {
		return s.GetID() == "id1"
	})
	assert.Len(t, sessions, 1)
}

func TestGetAllSessions(t *testing.T) {
	h := &haus{
		sessionsMutex: &sync.RWMutex{},
		sessions: map[*Session]struct{}{
			{
				id: "id1",
			}: {},
			{
				id: "id2",
			}: {},
		},
	}
	assert.Len(t, h.getAllSessions(), 2)
}

func TestRegisterUnregisterSession(t *testing.T) {
	h := &haus{
		sessionsMutex:         &sync.RWMutex{},
		sessions:              map[*Session]struct{}{},
		sessionsWithTagsMutex: &sync.RWMutex{},
		sessionsWithTags:      map[string]map[*Session]struct{}{},
		handlers: &handlers{
			logHandler: func(s *Session, log string) {},
		},
	}

	session1 := &Session{id: "1"}
	h.registerSession(session1, map[string]struct{}{"tag1": {}})
	h.registerSession(&Session{id: "2"}, map[string]struct{}{"tag2": {}})
	assert.Len(t, h.getAllSessions(), 2)

	sessions := h.filterSessionsByTag("tag1")
	assert.Len(t, sessions, 1)

	h.unregisterSession(session1)
	assert.Len(t, h.getAllSessions(), 1)

	sessions = h.filterSessionsByTag("tag1")
	assert.Len(t, sessions, 0)
}
