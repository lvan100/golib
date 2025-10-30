/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package httputil

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// Message represents a single message unit read from the stream.
type Message struct {
	data string
	err  error
}

// Stream manages streaming data asynchronously from an HTTP response.
// It supports safe concurrent use and can be closed idempotently.
type Stream struct {
	msgs   chan Message
	curr   Message
	closed atomic.Bool
	done   chan struct{}
}

// NewStream creates and initializes a new Stream instance.
func NewStream() *Stream {
	return &Stream{
		msgs: make(chan Message),
		done: make(chan struct{}),
	}
}

// Close closes the Stream safely (idempotent).
// It ensures that the internal channels are closed only once.
func (s *Stream) Close() {
	if s.closed.CompareAndSwap(false, true) {
		close(s.done)
		close(s.msgs)
	}
}

// Text returns the latest data read from the stream.
func (s *Stream) Text() string {
	return s.curr.data
}

// Error returns the last error encountered by the stream.
func (s *Stream) Error() error {
	return s.curr.err
}

// Next waits for the next data from the stream until timeout.
// Returns true if a new data item is available, false otherwise.
func (s *Stream) Next(ctx context.Context, timeout time.Duration) bool {
	if s.closed.Load() {
		return false
	}

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	select {
	case <-ctx.Done():
		s.curr.data = ""
		s.curr.err = ctx.Err()
		return false
	case s.curr, _ = <-s.msgs:
		if s.curr.err != nil {
			if s.curr.err == io.EOF {
				s.curr.err = nil
				return false
			}
			return false
		}
		return true
	}
}

// send pushes a Message into the internal channel.
// Returns false if the stream is closed or done.
func (s *Stream) send(msg Message) bool {
	if s.closed.Load() {
		return false
	}
	select {
	case <-s.done:
		return false
	case s.msgs <- msg:
		return true
	}
}

// RequestContext holds the context information for an HTTP request.
type RequestContext struct {
	Path   string
	Header http.Header
	Config map[string]string
}

// RequestOption is a function type that modifies the RequestContext.
type RequestOption func(info *RequestContext)

// WithHeader sets the given http.Header to the RequestContext.
func WithHeader(header http.Header) RequestOption {
	return func(meta *RequestContext) {
		maps.Copy(meta.Header, header)
	}
}

// WithConfig sets the given map to the RequestContext.
func WithConfig(config map[string]string) RequestOption {
	return func(meta *RequestContext) {
		maps.Copy(meta.Config, config)
	}
}

// Transport defines a customizable HTTP transport interface.
type Transport interface {
	GetConn(target, schema string) Connection
}

// Connection defines a customizable HTTP executor interface.
// Implementing this interface allows users to provide their own
// HTTP execution logic (for example, to add retry, logging, or tracing).
type Connection interface {
	JSON(req *http.Request, meta RequestContext) (*http.Response, []byte, error)
	Stream(req *http.Request, meta RequestContext) (*http.Response, *Stream, error)
}

var _ Transport = (*DefaultTransport)(nil)
var _ Connection = (*DefaultConnection)(nil)

// DefaultTransport is the default implementation of Transport,
type DefaultTransport struct{}

// GetConn returns the default connection for the transport.
func (f *DefaultTransport) GetConn(target, schema string) Connection {
	return &DefaultConnection{
		Client: http.DefaultClient,
		Target: target,
		Scheme: schema,
	}
}

// DefaultConnection is the default implementation of Connection,
// which delegates to the standard library http.Client.
type DefaultConnection struct {
	Client *http.Client
	Target string
	Scheme string
}

// JSON executes the HTTP request using the embedded http.Client.
// It reads the entire response body into memory and returns both
// the *http.Response and the body as a byte slice.
//
// The response body is also replaced with a reusable buffer so that
// it can be read again by the caller if needed.
//
// Note: For very large responses, this may be memory intensive.
func (c *DefaultConnection) JSON(r *http.Request, meta RequestContext) (*http.Response, []byte, error) {
	r.Host = c.Target
	r.URL.Host = c.Target
	r.URL.Scheme = c.Scheme
	maps.Copy(r.Header, meta.Header)

	resp, err := c.Client.Do(r)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	// Reset the response body to allow it to be read again later.
	resp.Body = io.NopCloser(bytes.NewBuffer(b))
	return resp, b, nil
}

// Stream executes an HTTP request and continuously reads lines from the response body.
// Each line is sent into the returned Stream channel asynchronously.
func (c *DefaultConnection) Stream(r *http.Request, meta RequestContext) (*http.Response, *Stream, error) {
	r.Host = c.Target
	r.URL.Host = c.Target
	r.URL.Scheme = c.Scheme
	maps.Copy(r.Header, meta.Header)

	resp, err := c.Client.Do(r)
	if err != nil {
		return nil, nil, err
	}

	respStream := NewStream()
	go func() {
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			text := strings.TrimSpace(scanner.Text())
			if text == "" {
				continue
			}
			if !respStream.send(Message{data: text}) {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			respStream.send(Message{err: err})
		} else {
			respStream.send(Message{err: io.EOF})
		}
	}()
	return resp, respStream, nil
}

// NewRequest creates a new HTTP request with the given context, method,
// URL, protocol encoder, and request body.
func NewRequest(ctx context.Context, method string, url string, p Protocol, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		b, err := p.Encode(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	return req, nil
}

// JSONResponse executes the given HTTP request using the provided Connection,
// reads the response body, and unmarshal it into a value of type RespType.
func JSONResponse[RespType any](c Connection, r *http.Request, path string, opts ...RequestOption) (*http.Response, *RespType, error) {
	meta := RequestContext{
		Path:   path,
		Header: http.Header{},
		Config: map[string]string{},
	}
	for _, opt := range opts {
		opt(&meta)
	}
	resp, b, err := c.JSON(r, meta)
	if err != nil {
		return nil, nil, err
	}
	var ret RespType
	if err = json.Unmarshal(b, &ret); err != nil {
		return nil, nil, err
	}
	return resp, &ret, nil
}

// StreamResponse executes the given HTTP request using the provided Connection,
// and returns a Stream instance for streaming the response body.
func StreamResponse(c Connection, r *http.Request, path string, opts ...RequestOption) (*http.Response, *Stream, error) {
	meta := RequestContext{
		Path:   path,
		Header: http.Header{},
		Config: map[string]string{},
	}
	for _, opt := range opts {
		opt(&meta)
	}
	return c.Stream(r, meta)
}
