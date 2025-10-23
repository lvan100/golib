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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// HTTPClient defines a customizable HTTP executor interface.
// Implementing this interface allows users to provide their own
// HTTP execution logic (for example, to add retry, logging, or tracing).
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, []byte, error)
}

// DefaultHTTPClient is the default implementation of HTTPClient,
// which delegates to the standard library http.Client.
type DefaultHTTPClient struct {
	Client *http.Client
	Scheme string
	Host   string
}

// Do executes the HTTP request using the embedded http.Client.
// It reads the entire response body into memory and returns both
// the *http.Response and the body as a byte slice.
//
// The response body is also replaced with a reusable buffer so that
// it can be read again by the caller if needed.
//
// Note: For very large responses, this may be memory intensive.
func (c *DefaultHTTPClient) Do(r *http.Request) (*http.Response, []byte, error) {
	r.Host = c.Host
	r.URL.Host = c.Host
	r.URL.Scheme = c.Scheme
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

// NewRequest creates a new HTTP request with the given context, method,
// URL, protocol encoder, and request body.
func NewRequest(ctx context.Context, method string, urlPath string, p Protocol, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		b, err := p.Encode(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	return http.NewRequestWithContext(ctx, method, urlPath, reader)
}

// JSONResponse executes the given HTTP request using the provided HTTPClient,
// reads the response body, and unmarshal it into a value of type RespType.
func JSONResponse[RespType any](c HTTPClient, r *http.Request) (*http.Response, *RespType, error) {
	resp, b, err := c.Do(r)
	if err != nil {
		return nil, nil, err
	}
	var ret *RespType
	err = json.Unmarshal(b, &ret)
	if err != nil {
		return nil, nil, err
	}
	return resp, ret, nil
}
