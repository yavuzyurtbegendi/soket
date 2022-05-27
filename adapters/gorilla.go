package adapters

import (
	"time"

	"github.com/gorilla/websocket"
)

type Gorilla struct {
	conn *websocket.Conn
}

func (g *Gorilla) WriteMessage(messageType int, data []byte) error {
	return g.conn.WriteMessage(messageType, data)
}

func (g *Gorilla) SetWriteDeadline(t time.Time) error {
	return g.conn.SetWriteDeadline(t)
}

func (g *Gorilla) SetReadLimit(limit int64) {
	g.conn.SetReadLimit(limit)
}

func (g *Gorilla) SetReadDeadline(t time.Time) error {
	return g.conn.SetReadDeadline(t)
}

func (g *Gorilla) SetPingHandler(h func(appData string) error) {
	g.conn.SetPingHandler(h)
}

func (g *Gorilla) SetPongHandler(h func(appData string) error) {
	g.conn.SetPongHandler(h)
}

func (g *Gorilla) SetCloseHandler(h func(code int, text string) error) {
	g.conn.SetCloseHandler(h)
}

func (g *Gorilla) ReadMessage() (int, []byte, error) {
	return g.conn.ReadMessage()
}

func (g *Gorilla) Close() error {
	return g.conn.Close()
}
