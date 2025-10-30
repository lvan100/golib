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
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/lvan100/golib/testing/assert"
)

// ToString converts the given value to a string.
func ptr[T any](v T) *T {
	return &v
}

type LogTransport struct {
	DefaultTransport
}

func (c *LogTransport) GetConn(target, schema string) Connection {
	return &LogConnection{
		Connection: c.DefaultTransport.GetConn(target, schema),
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
	ServiceName string
}

type Item struct {
	ID int64 `json:"id"`
}

type Object struct {
	Item *Item  `json:"item"`
	Text string `json:"text"`
}

type HelloRequest struct {
	HelloRequestBody
	Int             int               `json:"int" query:"int"`
	String          string            `json:"string" query:"string"`
	IntPtr          *int              `json:"int_ptr" query:"int_ptr"`
	StringPtr       *string           `json:"string_ptr" query:"string_ptr"`
	IntSlice        []int             `json:"int_slice" query:"int_slice"`
	StringSlice     []string          `json:"string_slice" query:"string_slice"`
	ByteSlice       []byte            `json:"byte_slice" query:"byte_slice"`
	Object          *Object           `json:"object" query:"object"`
	ObjectSlice     []Object          `json:"object_slice" query:"object_slice"`
	IntStringMap    map[int]string    `json:"int_string_map" query:"int_string_map"`
	StringObjectMap map[string]Object `json:"string_object_map" query:"string_object_map"`
}

type HelloRequestBody struct{}

type HelloResponse struct {
	Message string `json:"message"`
}

// Hello sends a GET request to the /v1/hello endpoint with the given request body.
func (c *HelloClient) Hello(ctx context.Context, req *HelloRequest, opts ...RequestOption) (*http.Response, *HelloResponse, error) {
	m := url.Values{}

	m.Add("int", ToString(req.Int))
	m.Add("string", ToString(req.String))
	if req.IntPtr != nil {
		m.Add("int_ptr", ToString(*req.IntPtr))
	}
	if req.StringPtr != nil {
		m.Add("string_ptr", ToString(*req.StringPtr))
	}

	// Encode arrays using the repeated key format (e.g. a=1&a=2)
	for _, v := range req.IntSlice {
		m.Add("int_slice", ToString(v))
	}
	// Encode arrays using the repeated key format (e.g. a=1&a=2)
	for _, v := range req.StringSlice {
		m.Add("string_slice", ToString(v))
	}
	// Encode arrays using the repeated key format (e.g. a=1&a=2)
	if req.ByteSlice != nil {
		m.Add("byte_slice", base64.StdEncoding.EncodeToString(req.ByteSlice))
	}

	// Encode an array of objects using repeated keys with JSON values
	// e.g. items={"id":1,"name":"A"}&items={"id":2,"name":"B"}
	for _, v := range req.ObjectSlice {
		b, err := JSON.Encode(v)
		if err != nil {
			return nil, nil, err
		}
		m.Add("object_slice", string(b))
	}

	// Encode maps or structs as JSON strings (e.g. data={"id":1,"name":"Alice"})
	if req.Object != nil {
		b, err := JSON.Encode(req.Object)
		if err != nil {
			return nil, nil, err
		}
		m.Add("object", string(b))
	}
	// Encode maps or structs as JSON strings (e.g. data={"id":1,"name":"Alice"})
	if req.StringObjectMap != nil {
		b, err := JSON.Encode(req.StringObjectMap)
		if err != nil {
			return nil, nil, err
		}
		m.Add("string_object_map", string(b))
	}
	// Encode maps or structs as JSON strings (e.g. data={"id":1,"name":"Alice"})
	if req.IntStringMap != nil {
		b, err := JSON.Encode(req.IntStringMap)
		if err != nil {
			return nil, nil, err
		}
		m.Add("int_string_map", string(b))
	}

	path := "/v1/hello"
	urlPath := fmt.Sprintf("%s?%s", path, m.Encode())
	httpReq, err := NewRequest(ctx, "GET", urlPath, FORM, nil)
	if err != nil {
		return nil, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")
	conn := c.GetConn(c.ServiceName, "http")
	return JSONResponse[HelloResponse](conn, httpReq, path, opts...)
}

type StreamRequest struct {
	Prompt string `json:"prompt"`
}

// Stream sends a POST request to the /v1/stream endpoint with the given request body.
func (c *HelloClient) Stream(ctx context.Context, req *StreamRequest, opts ...RequestOption) (*http.Response, *Stream, error) {
	path := "/v1/stream"
	urlPath := fmt.Sprintf("%s", path)
	httpReq, err := NewRequest(ctx, "POST", urlPath, JSON, req)
	if err != nil {
		return nil, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	conn := c.GetConn(c.ServiceName, "http")
	return StreamResponse(conn, httpReq, path, opts...)
}

func TestHello(t *testing.T) {
	server := &http.Server{Addr: ":9090", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.URL.RawQuery)
		_ = r.Header.Write(os.Stdout)
		fmt.Println()
		_, _ = w.Write([]byte(fmt.Sprintf(`{"message": "hello %s"}`, r.URL.Query().Get("string"))))
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

	client := &HelloClient{
		Transport:   &LogTransport{},
		ServiceName: "127.0.0.1:9090",
	}

	_, data, err := client.Hello(context.Background(), &HelloRequest{
		Int:         5,
		String:      "world",
		IntPtr:      ptr(10),
		StringPtr:   ptr("message"),
		IntSlice:    []int{1, 2, 3},
		StringSlice: []string{"a", "b", "c"},
		ByteSlice:   []byte("hello world"),
		Object: &Object{
			Item: &Item{ID: 1010},
			Text: "message",
		},
		ObjectSlice: []Object{
			{
				Item: &Item{ID: 1010},
				Text: "message",
			},
			{
				Item: &Item{ID: 1010},
				Text: "message",
			},
		},
		IntStringMap: map[int]string{1: "one", 2: "two"},
		StringObjectMap: map[string]Object{
			"one": {
				Item: &Item{ID: 1010},
				Text: "message",
			},
			"two": {
				Item: &Item{ID: 1010},
				Text: "message",
			},
		},
	}, WithHeader(h))
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

	client := &HelloClient{
		Transport:   &LogTransport{},
		ServiceName: "127.0.0.1:9090",
	}

	_, resp, err := client.Stream(context.Background(), &StreamRequest{Prompt: "hello"}, WithHeader(h))
	defer resp.Close()
	assert.Error(t, err).Nil()

	for resp.Next(time.Second) {
		fmt.Println(resp.Text())
		//resp.Close()
	}
	fmt.Println(resp.Error())
	fmt.Println("done")
}
