package soket

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/soket/config"
	"github.com/stretchr/testify/assert"
)

var (
	errBreakFromLoop = errors.New("break_from_loop")
)

func TestGet(t *testing.T) {
	session := &Session{
		keyVal: map[string]interface{}{
			"q": 1,
		},
	}
	val, ok := session.Get("q")
	assert.Equal(t, val, 1)
	assert.True(t, ok)

	val, ok = session.Get("w")
	assert.Equal(t, val, nil)
	assert.False(t, ok)
}

func TestSet(t *testing.T) {
	session := &Session{}
	session.Set("q", 1)
	val, ok := session.Get("q")
	assert.Equal(t, val, 1)
	assert.True(t, ok)
}

func TestGetID(t *testing.T) {
	session := &Session{id: "id"}
	id := session.GetID()
	assert.Equal(t, id, "id")
}

func TestWriteMessageToPipe(t *testing.T) {
	session := &Session{
		messageQueue: make(chan *packet, 5),
		soket: &Soket{
			grace: grace{
				waitGroup: &sync.WaitGroup{},
				counter:   0,
			},
		},
	}

	session.writeMessageToPipe(&packet{eType: 1})
	session.writeMessageToPipe(&packet{eType: 1})
	session.writeMessageToPipe(&packet{eType: 1})

	messages := make([]*packet, 0)
	for i := 0; i < 3; i++ {
		messages = append(messages, <-session.messageQueue)
		session.decreaseCounter()
	}
	assert.Len(t, messages, 3)
	assert.Equal(t, atomic.LoadInt32(&session.soket.grace.counter), int32(0))
}

func TestCounters(t *testing.T) {
	session := &Session{
		soket: &Soket{
			grace: grace{
				waitGroup: &sync.WaitGroup{},
				counter:   0,
			},
		},
	}

	for i := 0; i < 3; i++ {
		session.increaseCounter()
	}
	assert.Equal(t, atomic.LoadInt32(&session.soket.grace.counter), int32(3))

	for i := 0; i < 3; i++ {
		session.decreaseCounter()
	}
	assert.Equal(t, atomic.LoadInt32(&session.soket.grace.counter), int32(0))
}

func TestWriteMessage(t *testing.T) {
	text := []byte("text")
	binary := []byte("binary")
	session := &Session{
		socketAdapter: &mockAdapter{},
		soket: &Soket{
			Config: &config.Config{
				WritePeriod: time.Hour,
			},
			handlers: &handlers{
				sentTextMessageHandler: func(s *Session, m []byte) {
					assert.Equal(t, text, m)
				},
				sentBinaryMessageHandler: func(s *Session, m []byte) {
					assert.Equal(t, binary, m)
				},
			},
		},
	}
	err := session.writeMessage(&packet{eType: websocket.TextMessage, message: text})
	assert.Nil(t, err)
	err = session.writeMessage(&packet{eType: websocket.BinaryMessage, message: binary})
	assert.Nil(t, err)
}

func TestWriteToSocket(t *testing.T) {
	text := []byte("text")
	session := &Session{
		messageQueue:  make(chan *packet, 5),
		socketAdapter: &mockAdapter{},
		soket: &Soket{
			grace: grace{
				waitGroup: &sync.WaitGroup{},
				counter:   0,
			},
			Config: &config.Config{
				PingPeriod: time.Second,
			},
			handlers: &handlers{
				logHandler: func(s *Session, log string) {
					assert.Equal(t, log, "SENDING_MESSAGE >> Message: text Type: 1")
				},
				sentTextMessageHandler: func(s *Session, m []byte) {
					assert.Equal(t, m, text)
				},
				sentPingMessageHandler: func(s *Session, m []byte) {
					assert.Nil(t, m)
				},
			},
		},
	}
	session.increaseCounter()
	go session.writeToSocket()
	session.messageQueue <- &packet{eType: websocket.TextMessage, message: text}
	time.Sleep(twoSecondDuration)
	close(session.messageQueue)
}

func TestReadFromSocket(t *testing.T) {
	text := []byte("text")
	binary := []byte("binary")
	session := &Session{
		socketAdapter: &mockAdapter{
			[]packet{
				{eType: websocket.TextMessage, message: text},
				{eType: websocket.BinaryMessage, message: binary},
			},
		},
		soket: &Soket{
			Config: &config.Config{
				MaxMessageSize: 1,
			},
			handlers: &handlers{
				receivedTextMessageHandler: func(s *Session, m []byte) {
					assert.Equal(t, m, text)
				},
				receivedBinaryMessageHandler: func(s *Session, m []byte) {
					assert.Equal(t, m, binary)
				},
				errorHandler: func(ses *Session, err error) {
					assert.Equal(t, err, errBreakFromLoop)
				},
			},
		},
	}
	session.readFromSocket()
}

type mockAdapter struct {
	packets []packet
}

func (m *mockAdapter) SetReadLimit(limit int64) {}

func (m *mockAdapter) SetPingHandler(h func(appData string) error) {}

func (m *mockAdapter) SetPongHandler(h func(appData string) error) {}

func (m *mockAdapter) SetCloseHandler(h func(code int, text string) error) {}

func (m *mockAdapter) WriteMessage(messageType int, data []byte) error {
	return nil
}

func (m *mockAdapter) SetWriteDeadline(t time.Time) error {
	return nil
}

func (m *mockAdapter) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockAdapter) ReadMessage() (int, []byte, error) {
	if len(m.packets) == 0 {
		return 0, nil, errBreakFromLoop
	}
	var x packet
	x, m.packets = m.packets[0], m.packets[1:]
	return x.eType, x.message, nil
}

func (m *mockAdapter) Close() error {
	return nil
}

const (
	twoSecondDuration = 2 * time.Second
)
