package soket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
	"github.com/soket/adapters"
)

type ISession interface {
	get() *Session
	writeToSocket()
	readFromSocket()
	writeMessage(*packet) error
	writeMessageToPipe(*packet)
	increaseCounter()
	decreaseCounter()
	getInitialNotification() ([]byte, map[*Session]struct{})
	close()

	GetID() string
	Set(key string, value interface{})
	Get(key string) (value interface{}, exists bool)
}

type packet struct {
	message []byte
	eType   int
}

type Session struct {
	keyVal        map[string]interface{}
	request       *http.Request
	soket         *Soket
	socketAdapter adapters.Socket
	messageQueue  chan *packet
	tags          map[string]struct{}
	id            string
	closed        bool
}

func initSession(webSocket adapters.Socket, r *http.Request, s *Soket) (ISession, error) {
	uid, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	return &Session{
		id:            uid.String(),
		request:       r,
		soket:         s,
		socketAdapter: webSocket,
		messageQueue:  make(chan *packet, s.Config.MessageQueueSize),
	}, nil
}

func (s *Session) writeMessageToPipe(pck *packet) {
	s.increaseCounter()
	select {
	case s.messageQueue <- pck:
	default:
		s.soket.handlers.errorHandler(s, fmt.Errorf("message queue is full | MessageQueueSize: %d", s.soket.Config.MessageQueueSize))
		s.decreaseCounter()
	}
}

func (s *Session) increaseCounter() {
	s.soket.grace.waitGroup.Add(1)
	atomic.AddInt32(&s.soket.grace.counter, 1)
}

func (s *Session) decreaseCounter() {
	s.soket.grace.waitGroup.Done()
	atomic.AddInt32(&s.soket.grace.counter, -1)
}

func (s *Session) writeMessage(pck *packet) error {
	err := s.socketAdapter.SetWriteDeadline(time.Now().Add(s.soket.Config.WritePeriod))
	if err != nil {
		return err
	}
	err = s.socketAdapter.WriteMessage(pck.eType, pck.message)
	if err != nil {
		return err
	}
	switch pck.eType {
	case websocket.TextMessage:
		s.soket.handlers.sentTextMessageHandler(s, pck.message)
	case websocket.BinaryMessage:
		s.soket.handlers.sentBinaryMessageHandler(s, pck.message)
	case websocket.PingMessage:
		s.soket.handlers.sentPingMessageHandler(s, pck.message)
	}
	return nil
}

func (s *Session) get() *Session {
	return s
}

func (s *Session) getInitialNotification() ([]byte, map[*Session]struct{}) {
	initialPayload, _ := json.Marshal(map[string]string{
		"sessionId": s.id,
	})
	return initialPayload, map[*Session]struct{}{
		s: {},
	}
}

// this is a goroutine, fired from soket.go
func (s *Session) writeToSocket() {
	ticker := time.NewTicker(s.soket.Config.PingPeriod)
	defer ticker.Stop()
	for {
		select {
		case pck, ok := <-s.messageQueue:
			if !ok {
				return
			}
			s.soket.handlers.logHandler(s, fmt.Sprintf("SENDING_MESSAGE >> Message: %s Type: %d", string(pck.message), pck.eType))
			s.decreaseCounter()
			if err := s.writeMessage(pck); err != nil {
				s.soket.handlers.errorHandler(s, err)
				return
			}
		case <-ticker.C:
			// sometimes this ticks after the socket is closed, which generates "websocket: close sent"
			// I did not want to use !s.soket.haus.isOpen()
			if err := s.writeMessage(&packet{eType: websocket.PingMessage}); err != nil && err != websocket.ErrCloseSent {
				s.soket.handlers.errorHandler(s, err)
				return
			}
		}
	}
}

func (s *Session) readFromSocket() {
	s.socketAdapter.SetReadLimit(int64(s.soket.Config.MaxMessageSize))
	if err := s.socketAdapter.SetReadDeadline(time.Now().Add(s.soket.Config.PongPeriod)); err != nil {
		s.soket.handlers.errorHandler(s, err)
		return
	}
	s.socketAdapter.SetPingHandler(func(appName string) error {
		s.soket.handlers.pingHandler(s, appName)
		return nil
	})
	s.socketAdapter.SetPongHandler(func(appName string) error {
		if err := s.socketAdapter.SetReadDeadline(time.Now().Add(s.soket.Config.PongPeriod)); err != nil {
			s.soket.handlers.errorHandler(s, err)
			return err
		}
		s.soket.handlers.pongHandler(s, appName)
		return nil
	})
	s.socketAdapter.SetCloseHandler(func(code int, text string) error {
		s.soket.handlers.closeHandler(code, text)
		return nil
	})
	for {
		t, message, err := s.socketAdapter.ReadMessage()
		if err != nil {
			if c, k := err.(*websocket.CloseError); k {
				if c.Code == websocket.CloseGoingAway ||
					c.Code == websocket.CloseAbnormalClosure ||
					c.Code == websocket.CloseNoStatusReceived {
					break
				}
			}
			s.soket.handlers.errorHandler(s, err)
			break
		}
		switch t {
		case websocket.TextMessage:
			s.soket.handlers.receivedTextMessageHandler(s, message)
		case websocket.BinaryMessage:
			s.soket.handlers.receivedBinaryMessageHandler(s, message)
		}
	}
}

func (s *Session) close() {
	if err := s.socketAdapter.Close(); err != nil {
		s.soket.handlers.errorHandler(s, err)
	}
	s.closed = true
	close(s.messageQueue)
}

func (s *Session) GetID() string {
	return s.id
}

func (s *Session) Set(key string, value interface{}) {
	if s.keyVal == nil {
		s.keyVal = make(map[string]interface{})
	}
	s.keyVal[key] = value
}

func (s *Session) Get(key string) (value interface{}, exists bool) {
	if s.keyVal != nil {
		value, exists = s.keyVal[key]
	}
	return
}
