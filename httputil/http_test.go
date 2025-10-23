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
	"sync"
	"testing"
	"time"

	"github.com/lvan100/golib/testing/assert"
)

type HelloClient struct{}

// getClient returns the default HTTP client.
func (c *HelloClient) getClient() Client {
	return &DefaultClient{
		Client: http.DefaultClient,
		Scheme: "http",
		Host:   "127.0.0.1:9090",
	}
}

type HelloRequest struct {
	Name string `json:"name"`
}

type HelloResponse struct {
	Message string `json:"message"`
}

// Hello sends a GET request to the /v1/hello endpoint with the given request body.
func (c *HelloClient) Hello(ctx context.Context, req *HelloRequest) (*http.Response, *HelloResponse, error) {
	url := fmt.Sprintf("/v1/hello?name=%s", req.Name)
	httpReq, err := NewRequest(ctx, "GET", url, FORM, nil)
	if err != nil {
		return nil, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")
	return JSONResponse[HelloResponse](c.getClient(), httpReq)
}

type StreamRequest struct {
	Prompt string `json:"prompt"`
}

// Stream sends a POST request to the /v1/stream endpoint with the given request body.
func (c *HelloClient) Stream(ctx context.Context, req *StreamRequest) (*http.Response, *Stream, error) {
	url := fmt.Sprintf("/v1/stream")
	httpReq, err := NewRequest(ctx, "POST", url, JSON, req)
	if err != nil {
		return nil, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	return c.getClient().Stream(httpReq)
}

func TestHello(t *testing.T) {
	server := &http.Server{Addr: ":9090", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fmt.Sprintf(`{"message": "hello %s"}`, r.URL.Query().Get("name"))))
	})}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = server.ListenAndServe()
	}()
	time.Sleep(time.Millisecond * 100)

	client := &HelloClient{}
	_, data, err := client.Hello(context.Background(), &HelloRequest{Name: "world"})
	assert.Error(t, err).Nil()
	assert.That(t, data).Equal(&HelloResponse{Message: "hello world"})

	_ = server.Shutdown(context.Background())
	wg.Wait()
}

func TestStream(t *testing.T) {
	server := http.Server{Addr: ":9090", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := range 5 {
			_, _ = w.Write([]byte(fmt.Sprintf("%d: ", i)))
			_, _ = w.Write([]byte(`{"message": "hello world"}`))
			_, _ = w.Write([]byte("\n\n"))
			w.(http.Flusher).Flush()
			time.Sleep(time.Millisecond * 500)
		}
		fmt.Println("server done")
	})}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = server.ListenAndServe()
	}()
	time.Sleep(time.Millisecond * 100)

	client := &HelloClient{}
	_, resp, err := client.Stream(context.Background(), &StreamRequest{Prompt: "hello"})
	defer resp.Close()
	assert.Error(t, err).Nil()

	for resp.Next(time.Second) {
		fmt.Println(resp.Text())
		//resp.Close()
	}
	fmt.Println(resp.Error())
	fmt.Println("done")
}
