package adapters

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var (
	upgrader = &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
)

type Socket interface {
	WriteMessage(int, []byte) error
	SetWriteDeadline(time.Time) error
	SetReadLimit(int64)
	SetReadDeadline(time.Time) error
	SetPingHandler(func(string) error)
	SetPongHandler(func(string) error)
	SetCloseHandler(func(int, string) error)
	ReadMessage() (int, []byte, error)
	Close() error
}

func NewGorillaSocket(w http.ResponseWriter, r *http.Request) (Socket, error) {
	conn, err := upgrader.Upgrade(w, r, w.Header())
	if err != nil {
		return nil, err
	}
	return &Gorilla{conn: conn}, nil
}
