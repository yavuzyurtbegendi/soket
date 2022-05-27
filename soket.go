package soket

import (
	"net/http"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/soket/adapters"
	"github.com/soket/config"
)

type ISoket interface {
	HandleRequest(http.ResponseWriter, *http.Request, func(*Session)) error
	HandleRequestWithTags(http.ResponseWriter, *http.Request, map[string]struct{}, func(*Session)) error

	HandleConnect(sessionFunc)
	HandleDisconnect(sessionFunc)
	HandleError(sessionErrorFunc)
	HandleReceivedTextMessage(sessionMessageFunc)
	HandleReceivedBinaryMessage(sessionMessageFunc)
	HandleSentTextMessage(sessionMessageFunc)
	HandleSentBinaryMessage(sessionMessageFunc)
	HandleSentPingMessage(sessionMessageFunc)
	HandleClose(closeFunc)

	// TEXT MESSAGES
	BroadcastTextToAll([]byte)
	BroadcastTextTo([]byte, map[*Session]struct{})
	BroadcastTextToTag([]byte, string)
	BroadcastTextWithFiltering([]byte, func(*Session) bool)

	// BINARY MESSAGES
	BroadcastBinaryToAll([]byte)
	BroadcastBinartyTo([]byte, map[*Session]struct{})
	BroadcastBinaryToTag([]byte, string)
	BroadcastBinaryWithFiltering([]byte, func(*Session) bool)

	BroadcastExit()
	BroadcastExitTo(map[*Session]struct{})

	GetAllSessions() map[*Session]struct{}

	Shutdown()
}

type grace struct {
	waitGroup *sync.WaitGroup
	counter   int32
}

type Soket struct {
	Config   *config.Config
	haus     IHaus
	handlers *handlers
	grace    grace
}

type handlers struct {
	closeHandler                 closeFunc
	pingHandler                  pingPongFunc
	pongHandler                  pingPongFunc
	connectHandler               sessionFunc
	disconnectHandler            sessionFunc
	errorHandler                 sessionErrorFunc
	receivedTextMessageHandler   sessionMessageFunc
	receivedBinaryMessageHandler sessionMessageFunc
	sentTextMessageHandler       sessionMessageFunc
	sentBinaryMessageHandler     sessionMessageFunc
	sentPingMessageHandler       sessionMessageFunc
	logHandler                   logFunc
}

type closeFunc func(int, string)
type logFunc func(*Session, string)
type pingPongFunc func(*Session, string)
type sessionFunc func(*Session)
type sessionErrorFunc func(*Session, error)
type sessionMessageFunc func(*Session, []byte)

// New creates a new soket instance.
func New(configs ...config.ConfigParam) ISoket {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	conf := config.LoadConfig(configs)
	handlers := &handlers{
		closeHandler:      func(int, string) {},
		pingHandler:       func(*Session, string) {},
		pongHandler:       func(*Session, string) {},
		connectHandler:    func(*Session) {},
		disconnectHandler: func(*Session) {},
		logHandler: func(session *Session, message string) {
			log.Info().Str("ID", session.GetID()).Msg(message)
		},
		errorHandler: func(ses *Session, err error) {
			_, fn, line, _ := runtime.Caller(1)
			log.Error().Err(err).
				Int("line", line).Str("function", fn).
				Str("session_id", ses.GetID()).Msg("Error captured >>")
		},
		receivedTextMessageHandler:   func(*Session, []byte) {},
		receivedBinaryMessageHandler: func(*Session, []byte) {},
		sentTextMessageHandler:       func(*Session, []byte) {},
		sentBinaryMessageHandler:     func(*Session, []byte) {},
		sentPingMessageHandler:       func(*Session, []byte) {},
	}
	var waitGroup sync.WaitGroup
	return &Soket{
		haus:     newHaus(conf, handlers),
		Config:   conf,
		handlers: handlers,
		grace: grace{
			waitGroup: &waitGroup,
			counter:   0,
		},
	}
}

// HandleRequest upgrades http requests to websocket connections, returns the session from the inner function.
func (s *Soket) HandleRequest(w http.ResponseWriter, r *http.Request, f func(*Session)) error {
	return s.HandleRequestWithTags(w, r, nil, f)
}

// HandleRequestWithTags upgrades http requests to websocket connections, returns the session from the inner function. You can supply tags if you like to filter quickly.
func (s *Soket) HandleRequestWithTags(w http.ResponseWriter, r *http.Request, tags map[string]struct{}, f func(*Session)) error {
	if !s.haus.isOpen() {
		return nil
	}
	gorillaSocket, err := adapters.NewGorillaSocket(w, r)
	if err != nil {
		return err
	}

	session, err := initSession(gorillaSocket, r, s)
	if err != nil {
		return err
	}

	f(session.get())

	s.haus.registerSession(session.get(), tags)

	s.handlers.connectHandler(session.get())

	go session.writeToSocket()

	// notify client about the id of the session
	s.BroadcastTextTo(session.getInitialNotification())

	session.readFromSocket()

	session.close()

	s.haus.unregisterSession(session.get())

	s.handlers.disconnectHandler(session.get())

	return nil
}

// HandleConnect will be fired after upgrading request to a websocket connection.
func (s *Soket) HandleConnect(f sessionFunc) {
	s.handlers.connectHandler = f
}

// HandleDisconnect will be fired after disconnecting from the connection.
func (s *Soket) HandleDisconnect(f sessionFunc) {
	s.handlers.disconnectHandler = f
}

// HandleError will handle errors happened in the lifecycle of a websocket.
func (s *Soket) HandleError(f sessionErrorFunc) {
	s.handlers.errorHandler = f
}

// HandleReceivedTextMessage will be fired after receiving a message as byte. This also has the related session.
func (s *Soket) HandleReceivedTextMessage(f sessionMessageFunc) {
	s.handlers.receivedTextMessageHandler = f
}

// HandleReceivedBinaryMessage will be fired after receiving a message as byte. This also has the related session.
func (s *Soket) HandleReceivedBinaryMessage(f sessionMessageFunc) {
	s.handlers.receivedBinaryMessageHandler = f
}

// HandleSentTextMessage will be fired after the text message is sent.
func (s *Soket) HandleSentTextMessage(f sessionMessageFunc) {
	s.handlers.sentTextMessageHandler = f
}

// HandleSentBinaryMessage will be fired after the message is sent.
func (s *Soket) HandleSentBinaryMessage(f sessionMessageFunc) {
	s.handlers.sentBinaryMessageHandler = f
}

// HandleSentPingMessage will be fired after pinging.
func (s *Soket) HandleSentPingMessage(f sessionMessageFunc) {
	s.handlers.sentPingMessageHandler = f
}

// HandleClose will be fired after the connection is closed.
func (s *Soket) HandleClose(f closeFunc) {
	s.handlers.closeHandler = f
}

// BroadcastExit broadcasts exit to every registered session.
func (s *Soket) BroadcastExit() {
	allSessions := s.haus.getAllSessions()
	s.haus.broadcastTo(allSessions, &packet{
		eType: websocket.CloseMessage,
	})
}

// BroadcastExitTo broadcasts exit to only selected sessions.
func (s *Soket) BroadcastExitTo(sessions map[*Session]struct{}) {
	s.haus.broadcastTo(sessions, &packet{
		eType: websocket.CloseMessage,
	})
}

// BroadcastTextToAll broadcasts text message to every registered session.
func (s *Soket) BroadcastTextToAll(message []byte) {
	allSessions := s.haus.getAllSessions()
	s.haus.broadcastTo(allSessions, &packet{
		eType:   websocket.TextMessage,
		message: message,
	})
}

// BroadcastTextTo broadcasts text to only selected sessions.
func (s *Soket) BroadcastTextTo(message []byte, sessions map[*Session]struct{}) {
	s.haus.broadcastTo(sessions, &packet{
		eType:   websocket.TextMessage,
		message: message,
	})
}

// BroadcastTextToTag broadcasts text to sessions with tags.
func (s *Soket) BroadcastTextToTag(message []byte, topic string) {
	taggedSessions := s.haus.filterSessionsByTag(topic)
	s.haus.broadcastTo(taggedSessions, &packet{
		eType:   websocket.TextMessage,
		message: message,
	})
}

// BroadcastTextWithFiltering broadcasts text to sessions that match with specified filter.
func (s *Soket) BroadcastTextWithFiltering(message []byte, filter func(*Session) bool) {
	filteredSessions := s.haus.filterSessions(filter)
	s.haus.broadcastTo(filteredSessions, &packet{
		eType:   websocket.TextMessage,
		message: message,
	})
}

// BroadcastBinaryToAll broadcasts binary message to all connected sessions.
func (s *Soket) BroadcastBinaryToAll(message []byte) {
	allSessions := s.haus.getAllSessions()
	s.haus.broadcastTo(allSessions, &packet{
		eType:   websocket.BinaryMessage,
		message: message,
	})
}

// BroadcastBinartyTo broadcasts binary message to only selected sessions.
func (s *Soket) BroadcastBinartyTo(message []byte, sessions map[*Session]struct{}) {
	s.haus.broadcastTo(sessions, &packet{
		eType:   websocket.BinaryMessage,
		message: message,
	})
}

// BroadcastBinaryToTag broadcasts binary message to sessions with tags.
func (s *Soket) BroadcastBinaryToTag(message []byte, topic string) {
	taggedSessions := s.haus.filterSessionsByTag(topic)
	s.haus.broadcastTo(taggedSessions, &packet{
		eType:   websocket.BinaryMessage,
		message: message,
	})
}

// BroadcastBinaryWithFiltering broadcasts binary message to sessions that match with specified filter.
func (s *Soket) BroadcastBinaryWithFiltering(message []byte, filter func(*Session) bool) {
	filteredSessions := s.haus.filterSessions(filter)
	s.haus.broadcastTo(filteredSessions, &packet{
		eType:   websocket.BinaryMessage,
		message: message,
	})
}

// GetAllSessions returns all the available sessions.
func (s *Soket) GetAllSessions() map[*Session]struct{} {
	return s.haus.getAllSessions()
}

// Shutdown gracefully shutdowns the server.
func (s *Soket) Shutdown() {
	s.haus.close()
	s.BroadcastExit()
	go func() {
		var lastCount int32
		for {
			count := atomic.LoadInt32(&s.grace.counter)
			if count != lastCount {
				log.Info().Int32("Count", count).Msg("Jobs Remaining >> ")
				lastCount = count
			}
			time.Sleep(shutdownSleepSpan)
		}
	}()
	s.grace.waitGroup.Wait()
}

const (
	shutdownSleepSpan = 500 * time.Millisecond
)
