package websocket

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// constants for action type
const (
	publish     = "publish"
	subscribe   = "subscribe"
	unsubscribe = "unsubscribe"
)

const (
	ErrInvalidMessage       = "Server: Invalid msg"
	ErrActionUnrecognizable = "Server: Action unrecognized"
)

type Server struct {
	Subscriptions          Subscription
	subscriptionsMutex     sync.Mutex
	sendMu                 sync.Mutex
	receiveMu              sync.Mutex
	ReceiveMessageCbFunc   func(conn *websocket.Conn, clientID string, msg []byte)
	CbSubFunction          func(clientId string, m Message)
	CbUnSubFunction        func(clientId string, m Message)
	CbRemoveClientFunction func(clientId string)
}

func (s *Server) Send(conn *websocket.Conn, message string) {
	// send simple message
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	conn.WriteMessage(websocket.TextMessage, []byte(message))
}

func (s *Server) SendWithWait(cId string, conn *websocket.Conn, message string, wg *sync.WaitGroup) {
	// send simple message
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	conn.WriteMessage(websocket.TextMessage, []byte(message))

	// set the task as done
	wg.Done()
}

func (s *Server) RemoveClient(clientID string) {
	s.subscriptionsMutex.Lock()
	defer s.subscriptionsMutex.Unlock()
	// loop all topics
	for _, client := range s.Subscriptions {
		// delete the client from all the topic's client map
		delete(client, clientID)
	}

	if s.CbRemoveClientFunction != nil {
		s.CbRemoveClientFunction(clientID)
	}
}

func (s *Server) ProcessMessage(conn *websocket.Conn, clientID string, msg []byte) *Server {
	// parse message
	m := Message{}
	if err := json.Unmarshal(msg, &m); err != nil {
		s.Send(conn, ErrInvalidMessage)
	}

	// convert all action to lowercase and remove whitespace
	action := strings.TrimSpace(strings.ToLower(m.Action))

	switch action {
	// case publish:
	// 	s.Publish(m.Topic, []byte(m.Message))

	case subscribe:
		s.Subscribe(conn, clientID, m.Topic)
		if s.CbSubFunction != nil {
			s.CbSubFunction(clientID, m)
		}
	case unsubscribe:
		s.Unsubscribe(clientID, m.Topic)
		if s.CbUnSubFunction != nil {
			s.CbUnSubFunction(clientID, m)
		}
	default:
		s.Send(conn, ErrActionUnrecognizable)
	}

	return s
}

// Publish sends a message to all subscribing clients of a topic
func (s *Server) Publish(fn Fcondition, topic string, message []byte) {
	// if topic does not exist, stop the process
	if _, exist := s.Subscriptions[topic]; !exist {
		return
	}

	// if topic exist
	client := s.Subscriptions[topic]
	// send the message to the clients
	var wg sync.WaitGroup
	for cId, conn := range client {
		if fn(cId, topic) {
			// add 1 job to wait group
			wg.Add(1)

			// send with goroutines
			go s.SendWithWait(cId, conn, string(message), &wg)
		}
	}

	// wait until all goroutines jobs done
	wg.Wait()
}

// Subscribe adds a client to a topic's client map
func (s *Server) Subscribe(conn *websocket.Conn, clientID string, topic string) {
	s.subscriptionsMutex.Lock()
	defer s.subscriptionsMutex.Unlock()
	// if topic exist, check the client map
	if _, exist := s.Subscriptions[topic]; exist {
		client := s.Subscriptions[topic]

		// if client already subbed, stop the process
		if _, subbed := client[clientID]; subbed {
			return
		}

		// if not subbed, add to client map
		client[clientID] = conn
		return
	}

	// if topic does not exist, create a new topic
	newClient := make(Client)
	s.Subscriptions[topic] = newClient

	// add the client to the topic
	s.Subscriptions[topic][clientID] = conn
	fmt.Println("total subscriber to topic", topic, len(s.Subscriptions[topic]))
}

// Unsubscribe removes a clients from a topic's client map
func (s *Server) Unsubscribe(clientID string, topic string) {
	s.subscriptionsMutex.Lock()
	defer s.subscriptionsMutex.Unlock()
	// if topic exist, check the client map
	if _, exist := s.Subscriptions[topic]; exist {
		client := s.Subscriptions[topic]

		// remove the client from the topic's client map
		delete(client, clientID)
	}
	fmt.Println("total subscriber to topic", topic, len(s.Subscriptions[topic]))
}
