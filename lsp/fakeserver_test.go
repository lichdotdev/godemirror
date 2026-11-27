package lsp

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"
)

// fakeServer is a language server in the test's process, at the other end
// of a pipe: it answers requests with the handlers it is given and keeps
// every message it reads.
type fakeServer struct {
	t    *testing.T
	conn *conn
	// caps is what it answers initialize with.
	caps     map[string]any
	handlers map[string]func(params json.RawMessage) any

	mu   sync.Mutex
	got  []message
	seen chan struct{}
}

func newFakeServer(t *testing.T, caps map[string]any) *fakeServer {
	return &fakeServer{t: t, caps: caps, handlers: map[string]func(json.RawMessage) any{}, seen: make(chan struct{}, 1000)}
}

// connect starts the server and a Client talking to it.
func (s *fakeServer) connect(cfg Config) *Client {
	s.t.Helper()
	client, server := net.Pipe()
	s.conn = newConn(server)
	s.conn.handle = func(method string, params json.RawMessage) (any, *ResponseError) {
		s.keep(message{Method: method, Params: params})
		if method == "initialize" {
			return map[string]any{"capabilities": s.caps}, nil
		}
		if h := s.handlers[method]; h != nil {
			return h(params), nil
		}
		return nil, nil
	}
	s.conn.notify = func(method string, params json.RawMessage) {
		s.keep(message{Method: method, Params: params})
	}
	go s.conn.run()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := Connect(ctx, client, cfg)
	if err != nil {
		s.t.Fatal(err)
	}
	s.t.Cleanup(func() { _ = c.Close() })
	return c
}

func (s *fakeServer) keep(m message) {
	s.mu.Lock()
	s.got = append(s.got, m)
	s.mu.Unlock()
	s.seen <- struct{}{}
}

// wait waits until the server has read a message of method, and returns
// the params of the last one.
func (s *fakeServer) wait(method string) json.RawMessage {
	s.t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		if p, ok := s.last(method); ok {
			return p
		}
		select {
		case <-s.seen:
		case <-deadline:
			s.t.Fatalf("the server never got %s", method)
		}
	}
}

func (s *fakeServer) last(method string) (json.RawMessage, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.got) - 1; i >= 0; i-- {
		if s.got[i].Method == method {
			return s.got[i].Params, true
		}
	}
	return nil, false
}

// all are the params of every message of method read so far.
func (s *fakeServer) all(method string) []json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []json.RawMessage
	for _, m := range s.got {
		if m.Method == method {
			out = append(out, m.Params)
		}
	}
	return out
}

// publish sends textDocument/publishDiagnostics.
func (s *fakeServer) publish(p PublishDiagnostics) {
	s.t.Helper()
	if err := s.conn.send(nil, "textDocument/publishDiagnostics", p); err != nil {
		s.t.Fatal(err)
	}
}

// request sends the client a request and returns its raw result.
func (s *fakeServer) request(method string, params any) json.RawMessage {
	s.t.Helper()
	var raw json.RawMessage
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.conn.call(ctx, method, params, &raw); err != nil {
		s.t.Fatalf("%s: %v", method, err)
	}
	return raw
}

func unmarshal[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
