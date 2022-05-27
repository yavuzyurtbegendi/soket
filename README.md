# soket

Soket is a websocket implementation of [github.com/gorilla/websocket](https://github.com/gorilla/websocket)

Some features,
* You can broadcast to sessions with filters, tags and to all.
* Ping/pong and session timeouts will be handled by Soket.
* You can set key/value datas on a session.
* This library can be ported with any web framework. Only request and response is needed.

### Using With Echo
---
> You can find detailed example in examples/echo.go. Run `make run-echo`

```golang
type Name struct {
	Data string `json:"data"`
}

func main() {
	e := echo.New()
	s := soket.New()

	e.GET("/ws", func(c echo.Context) error {
            // In order to upgrade to a websocket, only response and request is necessary.
		return s.HandleRequest(c.Response().Writer, c.Request(), func(session *soket.Session) {
		})
	})

        // This is fired when a client sends a message.
	s.HandleReceivedTextMessage(func(senderSession *soket.Session, msg []byte) {
            // This broadcasts to all subscribed websockets.
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

        // This gracefully shutdowns the socket server
	s.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}

```

### Documentation
---

```golang
func New(configs ...config.ConfigParam) ISoket
```
New creates a new soket instance.
<br /><br />

```golang
func HandleRequest(w http.ResponseWriter, r *http.Request, f func(*Session)) error
```
Upgrades http requests to websocket connections, returns the session from the inner function.
<br /><br />

```golang
func HandleRequestWithTags(w http.ResponseWriter, r *http.Request, tags map[string]struct{}, f func(*Session)) error
```
Upgrades http requests to websocket connections, returns the session from the inner function. You can supply tags if you like to filter quickly. Check `BroadcastTextToTag` for broadcasting via a tag.
<br /><br />

```golang
func HandleConnect(f func(*Session))
```
This will be fired after upgrading request to a websocket connection.
<br /><br />

```golang
func HandleDisconnect(f func(*Session))
```
This will be fired after disconnecting from the connection.
<br /><br />

```golang
func HandleError(f func(*Session, error))
```
This will handle errors happened in the lifecycle of a websocket.
<br /><br />

```golang
func HandleReceivedTextMessage(f func(*Session, []byte))
```
The received message will be returned as byte with the related session.
<br /><br />

```golang
func HandleReceivedBinaryMessage(f func(*Session, []byte))
```
The received message will be returned as byte with the related session.
<br /><br />

```golang
func HandleSentTextMessage(f func(*Session, []byte))
```
This will be fired after the message is sent.
<br /><br />

```golang
func HandleSentBinaryMessage(f func(*Session, []byte))
```
This will be fired after the message is sent.
<br /><br />

```golang
func HandleSentPingMessage(f func(*Session, []byte))
```
This will be fired after pinging.
<br /><br />

```golang
func HandleClose(f func(int, string))
```
This will be fired after the connection is closed.
<br /><br />

```golang
func BroadcastExit()
```
Broadcasts exit to every registered session.
<br /><br />

```golang
func BroadcastExitTo(sessions map[*Session]struct{})
```
Broadcasts exit to only selected sessions.
<br /><br />

```golang
func BroadcastTextToAll(message []byte)
```
Broadcasts text message to every registered session.
<br /><br />

```golang
func BroadcastTextTo(message []byte, sessions map[*Session]struct{})
```
Broadcasts text to only selected sessions.
<br /><br />

```golang
func BroadcastTextToTag(message []byte, topic string)
```
Broadcasts text to sessions with tags.
<br /><br />

```golang
func BroadcastTextWithFiltering(message []byte, filter func(*Session) bool) {
```
Broadcasts text to sessions that match with specified filter.
<br /><br />

```golang
func BroadcastBinaryToAll(message []byte)
```
Broadcasts binary message to all connected sessions.
<br /><br />

```golang
func BroadcastBinartyTo(message []byte, sessions map[*Session]struct{})
```
Broadcasts binary message to only selected sessions.
<br /><br />

```golang
func BroadcastBinaryToTag(message []byte, topic string)
```
Broadcasts binary message to sessions with tags.
<br /><br />

```golang
func BroadcastBinaryWithFiltering(message []byte, filter func(*Session) bool)
```
Broadcasts binary message to sessions that match with specified filter.
<br /><br />

```golang
func GetAllSessions() map[*Session]struct{}
```
Returns all the available sessions.
<br /><br />

```golang
func Shutdown()
```
Gracefully shutdowns the server.
<br /><br />

```golang
func WithMessageQueueSize(messageQueueSize int) ConfigParam
```
Messages to be sent are queued in a channel. Channel queue size tells channel how many message should we keep.
<br /><br />

```golang
func WithWritePeriod(writePeriod time.Duration) ConfigParam
```
How long writing to socket should wait?.
<br /><br />

```golang
func WithPingPeriod(pingPeriod time.Duration) ConfigParam
```
After opening a socket connection the socket needs to be kept alive. After some time we should do pinging.
<br /><br />

```golang
func WithPongPeriod(pongPeriod time.Duration) ConfigParam
```
After opening a socket connection the socket needs to be kept alive. After some time we should do ponging.
<br /><br />

```golang
func WithMaxMessageSize(maxMessageSize int) ConfigParam
```
Sets the maximum size in bytes for a message read