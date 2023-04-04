package websocket

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// I/O read buffer size
	readBufferSize = 1024
	// I/O write buffer size
	writeBufferSize = 1024
)

var server = &Server{Subscriptions: make(Subscription)}
var upgrader = websocket.Upgrader{
	ReadBufferSize:  readBufferSize,
	WriteBufferSize: writeBufferSize,
	CheckOrigin: func(r *http.Request) bool {
		// allow all origin
		return true
	},
}

func HandleWS(w http.ResponseWriter, r *http.Request) {
	// upgrades connection to websocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("failed upgrading connection"))
		return
	}
	defer conn.Close()

	// create new client id
	clientID := uuid.New().String()

	// greet the new client
	server.Send(conn, fmt.Sprintf("Server: Welcome! Your ID is %s", clientID))

	// create channel to signal client health
	done := make(chan struct{})

	go writePump(conn, clientID, done)
	readPump(conn, clientID, done)
}

func readPump(conn *websocket.Conn, clientID string, done chan<- struct{}) {
	// set limit, deadline to read & pong handler
	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// message handling
	for {
		// read incoming message
		_, msg, err := conn.ReadMessage()
		// if error occured
		if err != nil {
			// remove from the client
			server.RemoveClient(clientID)
			// set health status to unhealthy by closing channel
			close(done)
			// stop process
			break
		}

		// if no error, process incoming message
		// server.ProcessMessage(conn, clientID, msg)
		server.ReceiveMessageCbFunc(conn, clientID, msg)
	}
}

// writePump sends ping to the client
func writePump(conn *websocket.Conn, clientID string, done <-chan struct{}) {
	// create ping ticker
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// send ping message
			err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait))
			if err != nil {
				// if error sending ping, remove this client from the server
				server.RemoveClient(clientID)
				// stop sending ping
				return
			}
		case <-done:
			// if process is done, stop sending ping
			return
		}
	}
}

func Publish(fn Fcondition, topic string, message []byte) {
	server.Publish(fn, topic, message)
}

func GetServer() *Server {
	return server
}