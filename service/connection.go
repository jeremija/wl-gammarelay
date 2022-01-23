package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/jeremija/wl-gammarelay/types"
)

type connection struct {
	conn net.Conn

	decoderMu sync.Mutex
	decoder   *json.Decoder

	encoderMu sync.Mutex
	encoder   *json.Encoder

	subscriptionsMu sync.RWMutex
	subscriptions   map[types.SubscriptionKey]struct{}
}

func newConnection(netConn net.Conn) *connection {
	c := &connection{
		conn:    netConn,
		encoder: json.NewEncoder(netConn),
		decoder: json.NewDecoder(netConn),

		subscriptions: map[types.SubscriptionKey]struct{}{},
	}

	return c
}

func (c *connection) Close() error {
	err := c.conn.Close()
	if err != nil {
		return fmt.Errorf("closing connection: %w", err)
	}

	return nil
}

func (c *connection) Write(response types.Response) error {
	c.encoderMu.Lock()
	defer c.encoderMu.Unlock()

	if err := c.conn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		return fmt.Errorf("setting write deadline: %w", err)
	}

	if err := c.encoder.Encode(response); err != nil {
		return fmt.Errorf("encoding response: %w", err)
	}

	return nil
}

func (c *connection) WriteLogError(response types.Response) {
	if err := c.Write(response); err != nil {
		log.Printf("Write error: %s\n", err)
	}
}

func (c *connection) Read() (types.Request, error) {
	c.decoderMu.Lock()
	defer c.decoderMu.Unlock()

	var request types.Request

	if err := c.decoder.Decode(&request); err != nil {
		return types.Request{}, fmt.Errorf("decoding request: %w", err)
	}

	return request, nil
}

func (c *connection) Subscribe(keys []types.SubscriptionKey) {
	c.subscriptionsMu.Lock()
	defer c.subscriptionsMu.Unlock()

	for _, key := range keys {
		c.subscriptions[key] = struct{}{}
	}
}

func (c *connection) Unsubscribe(keys []types.SubscriptionKey) {
	c.subscriptionsMu.Lock()
	defer c.subscriptionsMu.Unlock()

	for _, key := range keys {
		delete(c.subscriptions, key)
	}
}

func (c *connection) IsSubscribed(key types.SubscriptionKey) bool {
	c.subscriptionsMu.Lock()
	defer c.subscriptionsMu.Unlock()

	_, ok := c.subscriptions[key]

	return ok
}
