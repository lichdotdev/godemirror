package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
	"sync"
)

// JSON-RPC 2.0 error codes the client sends or reads.
const (
	codeMethodNotFound   = -32601
	codeInternalError    = -32603
	codeRequestCancelled = -32800
)

// ErrClosed is what a request returns once the connection to the server is
// gone, closed by Close or by the server.
var ErrClosed = errors.New("lsp: connection closed")

// ResponseError is an error a server answered a request with.
type ResponseError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *ResponseError) Error() string { return fmt.Sprintf("lsp: %s (%d)", e.Message, e.Code) }

// message is any JSON-RPC message: a request has an ID and a Method, a
// notification only a Method, a response only an ID.
type message struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *ResponseError   `json:"error,omitempty"`
}

type response struct {
	result json.RawMessage
	err    *ResponseError
}

// conn is a JSON-RPC 2.0 connection framed by Content-Length headers, as
// the base protocol of LSP 3.17 frames it.
type conn struct {
	rw io.ReadWriteCloser
	// handle answers a request, on a goroutine of its own; notify takes a
	// notification on the reading goroutine, in order, and returns fast.
	handle func(method string, params json.RawMessage) (any, *ResponseError)
	notify func(method string, params json.RawMessage)

	wmu sync.Mutex
	mu  sync.Mutex
	// next is the ID of the next request, pending the requests waiting for
	// their response.
	next    int64
	pending map[int64]chan response
	done    chan struct{}
	err     error
}

func newConn(rw io.ReadWriteCloser) *conn {
	return &conn{rw: rw, pending: map[int64]chan response{}, done: make(chan struct{})}
}

// run reads messages until the stream ends, then fails every request still
// waiting.
func (c *conn) run() {
	r := bufio.NewReader(c.rw)
	var err error
	for {
		var body []byte
		if body, err = readFrame(r); err != nil {
			break
		}
		var m message
		// A body that is not JSON-RPC is dropped: the frame around it still
		// says where the next message starts.
		if json.Unmarshal(body, &m) == nil {
			c.dispatch(m)
		}
	}
	c.mu.Lock()
	c.err = err
	c.pending = nil
	c.mu.Unlock()
	close(c.done)
}

func (c *conn) dispatch(m message) {
	switch {
	case m.Method != "" && m.ID != nil:
		// A request may take its time; the notifications after it, which
		// must stay in order, do not wait for it.
		go func() {
			result, rerr := c.handle(m.Method, m.Params)
			c.reply(*m.ID, result, rerr)
		}()
	case m.Method != "":
		c.notify(m.Method, m.Params)
	case m.ID != nil:
		id, err := strconv.ParseInt(string(*m.ID), 10, 64)
		if err != nil {
			return
		}
		c.mu.Lock()
		ch := c.pending[id]
		delete(c.pending, id)
		c.mu.Unlock()
		if ch != nil {
			ch <- response{m.Result, m.Error}
		}
	}
}

func (c *conn) reply(id json.RawMessage, result any, rerr *ResponseError) {
	m := message{ID: &id, Error: rerr}
	if rerr == nil {
		raw, err := json.Marshal(result)
		if err != nil {
			m.Error = &ResponseError{Code: codeInternalError, Message: err.Error()}
		} else {
			m.Result = raw
		}
	}
	// A reply that cannot be written finds the connection gone, which run
	// reports.
	_ = c.write(m)
}

// call sends a request and decodes its result into result, which may be
// nil. A cancelled ctx tells the server with $/cancelRequest and returns
// ctx.Err().
func (c *conn) call(ctx context.Context, method string, params, result any) error {
	c.mu.Lock()
	if c.pending == nil {
		c.mu.Unlock()
		return ErrClosed
	}
	c.next++
	id := c.next
	ch := make(chan response, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	raw := json.RawMessage(strconv.FormatInt(id, 10))
	if err := c.send(&raw, method, params); err != nil {
		c.forget(id)
		return err
	}
	select {
	case r := <-ch:
		if r.err != nil {
			return r.err
		}
		if result == nil || len(r.result) == 0 {
			return nil
		}
		return json.Unmarshal(r.result, result)
	case <-ctx.Done():
		c.forget(id)
		_ = c.send(nil, "$/cancelRequest", map[string]int64{"id": id})
		return ctx.Err()
	case <-c.done:
		return ErrClosed
	}
}

func (c *conn) forget(id int64) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

// send writes a request, or a notification when id is nil.
func (c *conn) send(id *json.RawMessage, method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.write(message{ID: id, Method: method, Params: raw})
}

func (c *conn) write(m message) error {
	m.JSONRPC = "2.0"
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if _, err := fmt.Fprintf(c.rw, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = c.rw.Write(body)
	return err
}

// readFrame reads one framed message's body: headers, a blank line, and
// Content-Length bytes.
func readFrame(r *bufio.Reader) ([]byte, error) {
	h, err := textproto.NewReader(r).ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	n, err := strconv.Atoi(h.Get("Content-Length"))
	if err != nil || n < 0 {
		return nil, fmt.Errorf("lsp: bad Content-Length %q", h.Get("Content-Length"))
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}
