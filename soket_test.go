package soket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

type Server struct {
	soket ISoket
}

func NewTestServer() *Server {
	return &Server{
		soket: New(),
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := s.soket.HandleRequest(w, r, func(s *Session) {})
	if err != nil {
		panic(err)
	}
}

func NewWebsocketClient(url string) (*websocket.Conn, error) {
	dialer := &websocket.Dialer{}
	client, _, err := dialer.Dial(strings.Replace(url, "http", "ws", 1), nil)
	return client, err
}

func TestSoket(t *testing.T) {
	soketMessage := []byte("message")
	messageContains := []string{"sessionId", "message"}

	testServer := NewTestServer()

	var sessionID string
	testServer.soket.HandleConnect(func(session *Session) {
		sessionID = session.GetID()
		assert.NotEmpty(t, sessionID)
	})

	testServer.soket.HandleDisconnect(func(s *Session) {
		assert.Equal(t, s.GetID(), sessionID)
	})

	testServer.soket.HandleReceivedTextMessage(func(session *Session, message []byte) {
		assert.Equal(t, soketMessage, message)
		session.writeMessageToPipe(&packet{
			message: message,
			eType:   websocket.TextMessage,
		})
	})

	testServer.soket.HandleSentTextMessage(func(session *Session, message []byte) {
		assert.NotEmpty(t, message)
	})

	testServer.soket.HandleSentPingMessage(func(session *Session, message []byte) {
		assert.Equal(t, session.GetID(), sessionID)
		assert.Nil(t, message)
	})

	server := httptest.NewServer(testServer)
	defer server.Close()

	websocketClient, err := NewWebsocketClient(server.URL)
	assert.Nil(t, err)

	err = websocketClient.WriteMessage(websocket.TextMessage, soketMessage)
	assert.Nil(t, err)

	for i := 0; i < 2; i++ {
		eType, eMessage, err := websocketClient.ReadMessage()
		assert.Equal(t, eType, 1)
		assert.Contains(t, string(eMessage), messageContains[i])
		assert.Nil(t, err)
	}

	websocketClient.Close()
}
