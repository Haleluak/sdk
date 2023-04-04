package websocket

import "github.com/gorilla/websocket"

// Subscription is a type for each string of topic and the clients that subscribe to it
type Subscription map[string]Client

// Client is a type that describe the clients' ID and their connection
type Client map[string]*websocket.Conn

type Fcondition func(string, string) bool

// Message is a struct for message to be sent by the client
type Message struct {
	Action  string   `json:"action"`
	Topic   string   `json:"event_name"`
	Message string   `json:"message"`
	Pairs   []string `json:"pairs"`
}
