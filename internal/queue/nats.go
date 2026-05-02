package queue

import (
	"context"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Client wraps a NATS connection + JetStream context with the Auditrail stream bootstrapped.
type Client struct {
	Conn   *nats.Conn
	JS     jetstream.JetStream
	Stream jetstream.Stream
}

// Connect dials NATS and ensures the EVENTS stream exists.
func Connect(ctx context.Context, url, streamName string) (*Client, error) {
	nc, err := nats.Connect(url, nats.Timeout(5*time.Second), nats.MaxReconnects(-1))
	if err != nil {
		return nil, err
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, err
	}

	subjects := []string{"events.>"}
	cfg := jetstream.StreamConfig{
		Name:      streamName,
		Subjects:  subjects,
		Retention: jetstream.LimitsPolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    7 * 24 * time.Hour,
		Discard:   jetstream.DiscardOld,
	}

	stream, err := js.Stream(ctx, streamName)
	if err != nil {
		if !errors.Is(err, jetstream.ErrStreamNotFound) {
			nc.Close()
			return nil, err
		}
		stream, err = js.CreateStream(ctx, cfg)
		if err != nil {
			nc.Close()
			return nil, err
		}
	}

	return &Client{Conn: nc, JS: js, Stream: stream}, nil
}

// Publish sends raw bytes to a subject with async publish.
func (c *Client) Publish(ctx context.Context, subject string, data []byte) error {
	_, err := c.JS.Publish(ctx, subject, data)
	return err
}

// Close terminates the NATS connection.
func (c *Client) Close() {
	if c.Conn != nil {
		c.Conn.Drain() //nolint:errcheck
	}
}
