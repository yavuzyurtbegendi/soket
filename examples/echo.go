package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/soket"
)

type Name struct {
	Data string `json:"data"`
}

func main() {

	e := echo.New()
	e.HideBanner = true

	mrecover := middleware.Recover()
	e.Use(mrecover)

	s := soket.New()

	e.GET("/", func(c echo.Context) error {
		http.ServeFile(c.Response().Writer, c.Request(), "html/index.html")
		return nil
	})

	e.GET("/ws", func(c echo.Context) error {
		return s.HandleRequest(c.Response().Writer, c.Request(), func(session *soket.Session) {
			// SEND DUMMY DATA
			go func(session *soket.Session) {
				messageData := 0
				for i := 0; i < 20; i++ {
					messageData++
					time.Sleep(1 * time.Second)
					data := Name{Data: fmt.Sprintf("Data: %d", messageData)}
					marshaledData, _ := json.Marshal(data)
					s.BroadcastTextTo(marshaledData, map[*soket.Session]struct{}{
						session: {},
					})
				}
				s.BroadcastExitTo(map[*soket.Session]struct{}{
					session: {},
				})
			}(session)
			// SEND DUMMY DATA
		})
	})

	s.HandleReceivedTextMessage(func(senderSession *soket.Session, msg []byte) {
		s.BroadcastTextToAll(msg)
	})

	go func() {
		if err := e.Start(":3001"); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("error shutting down the server")
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	// GRACEFULLY SHUTS DOWN THE SERVER
	s.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}
