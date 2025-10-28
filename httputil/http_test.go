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
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/lvan100/golib/testing/assert"
)

type LogTransport struct {
	DefaultTransport
}

func (c *LogTransport) GetConn() Connection {
	return &LogConnection{
		Connection: c.DefaultTransport.GetConn(),
	}
}

type LogConnection struct {
	Connection
}

// JSON executes the given HTTP request using the provided Client.
func (c *LogConnection) JSON(req *http.Request, meta RequestContext) (*http.Response, []byte, error) {
	fmt.Printf("%#v\n", meta)
	return c.Connection.JSON(req, meta)
}

// Stream executes the given HTTP request using the provided Client.
func (c *LogConnection) Stream(req *http.Request, meta RequestContext) (*http.Response, *Stream, error) {
	fmt.Printf("%#v\n", meta)
	return c.Connection.Stream(req, meta)
}

type HelloClient struct {
	Transport
}

type HelloRequest struct {
	Name string `json:"name"`
}

type HelloResponse struct {
	Message string `json:"message"`
}

// Hello sends a GET request to the /v1/hello endpoint with the given request body.
func (c *HelloClient) Hello(ctx context.Context, req *HelloRequest, opts ...RequestOption) (*http.Response, *HelloResponse, error) {
	path := "/v1/hello"
	url := fmt.Sprintf("%s?name=%s", path, req.Name)
	httpReq, err := NewRequest(ctx, "GET", url, FORM, nil)
	if err != nil {
		return nil, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")
	return JSONResponse[HelloResponse](c.GetConn(), httpReq, path, opts...)
}

type StreamRequest struct {
	Prompt string `json:"prompt"`
}

// Stream sends a POST request to the /v1/stream endpoint with the given request body.
func (c *HelloClient) Stream(ctx context.Context, req *StreamRequest, opts ...RequestOption) (*http.Response, *Stream, error) {
	path := "/v1/stream"
	url := fmt.Sprintf("%s", path)
	httpReq, err := NewRequest(ctx, "POST", url, JSON, req)
	if err != nil {
		return nil, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	return StreamResponse(c.GetConn(), httpReq, path, opts...)
}

func TestHello(t *testing.T) {
	server := &http.Server{Addr: ":9090", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.Header.Write(os.Stdout)
		fmt.Println()
		_, _ = w.Write([]byte(fmt.Sprintf(`{"message": "hello %s"}`, r.URL.Query().Get("name"))))
	})}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = server.ListenAndServe()
	}()
	time.Sleep(time.Millisecond * 100)

	h := http.Header{}
	h.Set("X-Request-ID", "12345678")

	client := &HelloClient{Transport: &LogTransport{}}
	_, data, err := client.Hello(context.Background(), &HelloRequest{Name: "world"}, SetHeader(h))
	assert.Error(t, err).Nil()
	assert.That(t, data).Equal(&HelloResponse{Message: "hello world"})

	_ = server.Shutdown(context.Background())
	wg.Wait()
}

func TestStream(t *testing.T) {
	server := http.Server{Addr: ":9090", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.Header.Write(os.Stdout)
		fmt.Println()
		for i := range 5 {
			_, _ = w.Write([]byte(fmt.Sprintf("%d: ", i)))
			_, _ = w.Write([]byte(`{"message": "hello world"}`))
			_, _ = w.Write([]byte("\n\n"))
			w.(http.Flusher).Flush()
			time.Sleep(time.Millisecond * 500)
		}
		fmt.Println()
		fmt.Println("server done")
	})}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = server.ListenAndServe()
	}()
	time.Sleep(time.Millisecond * 100)

	h := http.Header{}
	h.Set("X-Request-ID", "12345678")

	client := &HelloClient{Transport: &LogTransport{}}
	_, resp, err := client.Stream(context.Background(), &StreamRequest{Prompt: "hello"}, SetHeader(h))
	defer resp.Close()
	assert.Error(t, err).Nil()

	for resp.Next(time.Second) {
		fmt.Println(resp.Text())
		//resp.Close()
	}
	fmt.Println(resp.Error())
	fmt.Println("done")
}
