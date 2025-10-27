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
func (s *Stream) Next(timeout time.Duration) bool {
	if s.closed.Load() {
		return false
	}
	select {
	case <-time.After(timeout):
		s.curr.data = ""
		s.curr.err = context.DeadlineExceeded
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

// SetHeader sets the given http.Header to the RequestContext.
func SetHeader(header http.Header) RequestOption {
	return func(meta *RequestContext) {
		maps.Copy(meta.Header, header)
	}
}

// SetConfig sets the given map to the RequestContext.
func SetConfig(config map[string]string) RequestOption {
	return func(meta *RequestContext) {
		maps.Copy(meta.Config, config)
	}
}

// Client defines a customizable HTTP executor interface.
// Implementing this interface allows users to provide their own
// HTTP execution logic (for example, to add retry, logging, or tracing).
type Client interface {
	JSON(req *http.Request, meta RequestContext) (*http.Response, []byte, error)
	Stream(req *http.Request, meta RequestContext) (*http.Response, *Stream, error)
}

var _ Client = (*DefaultClient)(nil)

// DefaultClient is the default implementation of Client,
// which delegates to the standard library http.Client.
type DefaultClient struct {
	Client *http.Client
	Scheme string
	Host   string
}

// JSON executes the HTTP request using the embedded http.Client.
// It reads the entire response body into memory and returns both
// the *http.Response and the body as a byte slice.
//
// The response body is also replaced with a reusable buffer so that
// it can be read again by the caller if needed.
//
// Note: For very large responses, this may be memory intensive.
func (c *DefaultClient) JSON(r *http.Request, meta RequestContext) (*http.Response, []byte, error) {
	r.Host = c.Host
	r.URL.Host = c.Host
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
func (c *DefaultClient) Stream(r *http.Request, meta RequestContext) (*http.Response, *Stream, error) {
	r.Host = c.Host
	r.URL.Host = c.Host
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

// JSONResponse executes the given HTTP request using the provided Client,
// reads the response body, and unmarshal it into a value of type RespType.
func JSONResponse[RespType any](c Client, r *http.Request, path string, opts ...RequestOption) (*http.Response, *RespType, error) {
	meta := RequestContext{Path: path}
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

// StreamResponse executes the given HTTP request using the provided Client,
// and returns a Stream instance for streaming the response body.
func StreamResponse(c Client, r *http.Request, path string, opts ...RequestOption) (*http.Response, *Stream, error) {
	meta := RequestContext{Path: path}
	for _, opt := range opts {
		opt(&meta)
	}
	return c.Stream(r, meta)
}
