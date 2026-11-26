package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// shutdownWait is how long Close waits for the server to answer shutdown
// and exit before it kills the process.
const shutdownWait = 2 * time.Second

// Config is how to start and greet a server. The client knows no server:
// the caller names it, by hand or from the servers catalog.
type Config struct {
	// Command is the server's program and its arguments, for Start.
	Command []string
	// Env is added to the client's own environment for the server.
	Env []string
	// RootDir is the workspace the server works on, and the server's
	// working directory.
	RootDir string
	// InitializationOptions goes to the server with initialize, as is;
	// nil sends none.
	InitializationOptions json.RawMessage
	// Settings answers the server's workspace/configuration requests, the
	// section each asks for picked out of it, and goes with
	// workspace/didChangeConfiguration once initialized; nil sends none.
	Settings json.RawMessage
	// Stderr takes what the server writes to its standard error; nil drops
	// it.
	Stderr io.Writer
}

// Client is a connection to one language server. Its methods may be called
// from any goroutine.
type Client struct {
	conn *conn
	caps capabilities
	cfg  Config
	cmd  *exec.Cmd

	mu      sync.Mutex
	watches map[string]*watch
}

type watch struct{ f func(PublishDiagnostics) }

// Start runs cfg.Command in cfg.RootDir and connects to it over its
// standard input and output, as Connect does. ctx bounds the start and the
// initialize handshake, not the server's life: Close ends that.
func Start(ctx context.Context, cfg Config) (*Client, error) {
	if len(cfg.Command) == 0 {
		return nil, errors.New("lsp: no command")
	}
	cmd := exec.Command(cfg.Command[0], cfg.Command[1:]...)
	cmd.Dir = cfg.RootDir
	cmd.Env = append(os.Environ(), cfg.Env...)
	cmd.Stderr = cfg.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("lsp: starting %s: %w", cfg.Command[0], err)
	}
	c, err := Connect(ctx, stdio{stdout, stdin}, cfg)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, err
	}
	c.cmd = cmd
	return c, nil
}

// stdio is a server's standard output to read and standard input to write.
type stdio struct {
	io.ReadCloser
	in io.WriteCloser
}

func (s stdio) Write(p []byte) (int, error) { return s.in.Write(p) }

func (s stdio) Close() error { return errors.Join(s.in.Close(), s.ReadCloser.Close()) }

// Connect speaks LSP over rw, a server's stream, and runs the initialize
// handshake: it offers UTF-8 positions, which the Editor counts in, and
// falls back to what the server picks. Close closes rw.
func Connect(ctx context.Context, rw io.ReadWriteCloser, cfg Config) (*Client, error) {
	c := &Client{conn: newConn(rw), cfg: cfg, watches: map[string]*watch{}}
	c.conn.handle, c.conn.notify = c.handle, c.notify
	go c.conn.run()
	var result struct {
		Capabilities capabilities `json:"capabilities"`
	}
	if err := c.conn.call(ctx, "initialize", c.initializeParams(), &result); err != nil {
		_ = rw.Close()
		return nil, fmt.Errorf("lsp: initialize: %w", err)
	}
	c.caps = result.Capabilities
	if err := c.conn.send(nil, "initialized", struct{}{}); err != nil {
		_ = rw.Close()
		return nil, err
	}
	if cfg.Settings != nil {
		if err := c.conn.send(nil, "workspace/didChangeConfiguration", map[string]any{"settings": cfg.Settings}); err != nil {
			_ = rw.Close()
			return nil, err
		}
	}
	return c, nil
}

func (c *Client) initializeParams() map[string]any {
	root := FileURI(c.cfg.RootDir)
	p := map[string]any{
		"processId":  os.Getpid(),
		"clientInfo": map[string]string{"name": "godemirror"},
		"rootUri":    root,
		"workspaceFolders": []map[string]string{{
			"uri": root, "name": filepath.Base(c.cfg.RootDir),
		}},
		"capabilities": clientCapabilities,
	}
	if c.cfg.InitializationOptions != nil {
		p["initializationOptions"] = c.cfg.InitializationOptions
	}
	return p
}

// clientCapabilities are what the client tells a server it can take.
var clientCapabilities = map[string]any{
	"general": map[string]any{"positionEncodings": []PositionEncoding{UTF8, UTF16}},
	"workspace": map[string]any{
		"configuration":    true,
		"workspaceFolders": true,
	},
	"textDocument": map[string]any{
		"synchronization":    map[string]any{"didSave": true},
		"publishDiagnostics": map[string]any{"versionSupport": true},
		"completion": map[string]any{
			"completionItem": map[string]any{
				"snippetSupport":       false,
				"documentationFormat":  []string{"markdown", "plaintext"},
				"insertReplaceSupport": true,
				"resolveSupport":       map[string]any{"properties": []string{"documentation", "detail"}},
			},
			"contextSupport": true,
		},
		"hover":          map[string]any{"contentFormat": []string{"markdown", "plaintext"}},
		"definition":     map[string]any{"linkSupport": true},
		"typeDefinition": map[string]any{"linkSupport": true},
		"implementation": map[string]any{"linkSupport": true},
		"references":     map[string]any{},
	},
}

// Close asks the server to shut down and exit, as LSP's shutdown and exit,
// and closes the connection. A server started by Start that has not exited
// within a couple of seconds is killed.
func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownWait)
	defer cancel()
	err := c.conn.call(ctx, "shutdown", nil, nil)
	if err == nil {
		err = c.conn.send(nil, "exit", nil)
	}
	if c.cmd != nil {
		exited := make(chan error, 1)
		go func() { exited <- c.cmd.Wait() }()
		select {
		case <-exited:
		case <-ctx.Done():
			_ = c.cmd.Process.Kill()
			<-exited
		}
	}
	if cerr := c.conn.rw.Close(); err == nil {
		err = cerr
	}
	if errors.Is(err, ErrClosed) {
		return nil
	}
	return err
}

// Done is closed once the connection to the server is gone.
func (c *Client) Done() <-chan struct{} { return c.conn.done }

// Encoding is the position encoding the server picked: every Position the
// client sends or reads counts in it.
func (c *Client) Encoding() PositionEncoding {
	if c.caps.PositionEncoding == "" {
		return UTF16
	}
	return c.caps.PositionEncoding
}

// FileURI is the file URI of path, made absolute.
func FileURI(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

// FilePath is the path a file URI names, ok false for a URI of another
// scheme.
func FilePath(uri string) (path string, ok bool) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return "", false
	}
	return filepath.FromSlash(u.Path), true
}

// handle answers the requests a server sends the client.
func (c *Client) handle(method string, params json.RawMessage) (any, *ResponseError) {
	switch method {
	case "workspace/configuration":
		return c.configuration(params), nil
	case "workspace/workspaceFolders":
		return []map[string]string{{"uri": FileURI(c.cfg.RootDir), "name": filepath.Base(c.cfg.RootDir)}}, nil
	case "window/workDoneProgress/create", "client/registerCapability", "client/unregisterCapability":
		return nil, nil
	}
	return nil, &ResponseError{Code: codeMethodNotFound, Message: "method not found: " + method}
}

// configuration answers workspace/configuration: for each item, the
// section of Settings it names, dot by dot, or null.
func (c *Client) configuration(params json.RawMessage) []json.RawMessage {
	var p struct {
		Items []struct {
			Section string `json:"section"`
		} `json:"items"`
	}
	_ = json.Unmarshal(params, &p)
	out := make([]json.RawMessage, len(p.Items))
	for i, it := range p.Items {
		out[i] = section(c.cfg.Settings, it.Section)
	}
	return out
}

func (c *Client) notify(method string, params json.RawMessage) {
	if method != "textDocument/publishDiagnostics" {
		return
	}
	var p PublishDiagnostics
	if json.Unmarshal(params, &p) != nil {
		return
	}
	c.mu.Lock()
	w := c.watches[p.URI]
	c.mu.Unlock()
	if w != nil {
		w.f(p)
	}
}

// WatchDiagnostics hands every textDocument/publishDiagnostics for uri to f,
// on the goroutine that reads from the server, until stop is called; f
// returns fast. A later watch of the same uri replaces this one.
func (c *Client) WatchDiagnostics(uri string, f func(PublishDiagnostics)) (stop func()) {
	w := &watch{f}
	c.mu.Lock()
	c.watches[uri] = w
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.watches[uri] == w {
			delete(c.watches, uri)
		}
	}
}
