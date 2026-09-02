package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

type rpcMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("acp rpc error %d: %s", e.Code, e.Message)
}

// ErrClosed is returned once the connection has shut down.
var ErrClosed = errors.New("acp: connection closed")

type pending struct {
	result chan json.RawMessage
	fail   chan error
}

// conn multiplexes one newline-delimited JSON-RPC stream.
type conn struct {
	w   io.Writer
	enc *json.Encoder

	mu       sync.Mutex
	nextID   int
	waiting  map[int]*pending
	closed   bool
	closeErr error

	handler Handler

	done chan struct{}
}

func newConn(w io.Writer, r io.Reader, handler Handler) *conn {
	c := &conn{
		w:       w,
		enc:     json.NewEncoder(w),
		nextID:  1,
		waiting: make(map[int]*pending),
		handler: handler,
		done:    make(chan struct{}),
	}
	go c.readLoop(r)
	return c
}

func (c *conn) readLoop(r io.Reader) {
	defer close(c.done)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		c.dispatch(msg, line)
	}
	err := scanner.Err()
	if err == nil {
		err = ErrClosed
	}
	c.shutdown(err)
}

func (c *conn) dispatch(msg rpcMessage, raw []byte) {
	switch {
	case msg.Method != "" && msg.ID != nil:
		c.serveRequest(msg)
	case msg.Method != "":
		c.serveNotification(msg, raw)
	case msg.ID != nil:
		c.resolve(msg)
	}
}

func (c *conn) resolve(msg rpcMessage) {
	var id int
	if err := json.Unmarshal(*msg.ID, &id); err != nil {
		return
	}
	c.mu.Lock()
	p := c.waiting[id]
	delete(c.waiting, id)
	c.mu.Unlock()
	if p == nil {
		return
	}
	if msg.Error != nil {
		p.fail <- msg.Error
		return
	}
	p.result <- msg.Result
}

func (c *conn) call(ctx context.Context, method string, params, out any) error {
	c.mu.Lock()
	if c.closed {
		err := c.closeErr
		c.mu.Unlock()
		return err
	}
	id := c.nextID
	c.nextID++
	p := &pending{result: make(chan json.RawMessage, 1), fail: make(chan error, 1)}
	c.waiting[id] = p
	err := c.write(rpcMessage{JSONRPC: "2.0", ID: rawID(id), Method: method, Params: mustRaw(params)})
	c.mu.Unlock()
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.waiting, id)
		c.mu.Unlock()
		return ctx.Err()
	case err := <-p.fail:
		return err
	case res := <-p.result:
		if out == nil {
			return nil
		}
		return json.Unmarshal(res, out)
	}
}

func (c *conn) notify(method string, params any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return c.closeErr
	}
	return c.write(rpcMessage{JSONRPC: "2.0", Method: method, Params: mustRaw(params)})
}

// write requires c.mu held.
func (c *conn) write(msg rpcMessage) error {
	return c.enc.Encode(msg)
}

func (c *conn) reply(id json.RawMessage, result any, rpcErr *rpcError) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	msg := rpcMessage{JSONRPC: "2.0", ID: &id}
	if rpcErr != nil {
		msg.Error = rpcErr
	} else {
		msg.Result = mustRaw(result)
	}
	_ = c.write(msg)
}

func (c *conn) shutdown(err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.closeErr = err
	waiting := c.waiting
	c.waiting = map[int]*pending{}
	c.mu.Unlock()
	for _, p := range waiting {
		p.fail <- err
	}
}

func rawID(id int) *json.RawMessage {
	b, _ := json.Marshal(id)
	raw := json.RawMessage(b)
	return &raw
}

func mustRaw(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
