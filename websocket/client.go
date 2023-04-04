package websocket

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 5 * time.Second
	pongWait       = 10 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1000000 // 1MB
)

type Socket struct {
	conn            *websocket.Conn
	websocketDialer *websocket.Dialer
	OnMessage       func(data []byte)
	url             string
	mtx             sync.Mutex
	sendMu          *sync.Mutex // Prevent "concurrent write to websocket connection"
	receiveMu       *sync.Mutex
	shouldClose     bool
	status          chan struct{}
	stop            chan struct{}
}

func New(url string) Socket {
	return Socket{
		url:             url,
		websocketDialer: &websocket.Dialer{},
		sendMu:          &sync.Mutex{},
		receiveMu:       &sync.Mutex{},
		status:          make(chan struct{}, 1),
		stop:            make(chan struct{}, 1),
	}
}

func (c *Socket) Connect() error {
	if c.conn != nil {
		return nil
	}

	go c.managerConnection()

	err := c.connect()
	if err != nil {
		return err
	}

	return nil
}

func (c *Socket) connect() error {
	conn, res, err := websocket.DefaultDialer.Dial(c.url, nil)
	if err != nil {
		return fmt.Errorf("Error while connecting to server: %w", err)
	} else if res.StatusCode != 101 {
		return errors.New("server failed to switch protocols")
	}
	defer res.Body.Close()

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	c.conn = conn
	go func() {
		for {
			c.receiveMu.Lock()
			_, message, err := c.conn.ReadMessage()
			c.receiveMu.Unlock()
			if err != nil {
				c.status <- struct{}{}
				fmt.Println("read:", err)
				return
			}

			if c.OnMessage != nil {
				c.OnMessage(message)
			}
		}
	}()

	return nil
}

func (c *Socket) Close() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.stop <- struct{}{}
	c.close(false)
	close(c.stop)
	close(c.status)
}

func (c *Socket) reconnect() {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.shouldClose {
		return
	}

	fmt.Println("unexpected disconnect: reconnecting")
	c.close(true)

	c.connect()
}

func (c *Socket) Send(msg interface{}) error {
	if c.conn == nil {
		return nil
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.sendMu.Lock()
	err = c.conn.WriteMessage(websocket.TextMessage, data)
	c.sendMu.Unlock()
	return err

}

func (c *Socket) close(reconnect bool) {
	if c.conn == nil {
		return
	}

	if !reconnect {
		c.shouldClose = true
	}

	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

func (c *Socket) managerConnection() {
	heartbeat := time.NewTicker(pingPeriod)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.stop:
			return
		case <-c.status:
			time.Sleep(5 * time.Second)
			fmt.Println("reconnecting...")
			c.reconnect()
		case <-heartbeat.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Println("writemsg: ", err)
				return // return to break this goroutine triggeing cleanup
			}
		}

	}
}
