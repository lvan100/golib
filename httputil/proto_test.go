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
	"testing"

	"github.com/lvan100/golib/testing/assert"
)

func TestFORMProtocol(t *testing.T) {
	t.Run("nil value", func(t *testing.T) {
		b, err := FORM.Encode(nil)
		assert.That(t, err).Nil()
		assert.That(t, string(b)).Equal("")
	})

	t.Run("basic types", func(t *testing.T) {
		m := map[string]any{
			"string": "hello",
			"int":    123,
			"bool":   true,
			"float":  3.14,
		}
		b, err := FORM.Encode(m)
		assert.That(t, err).Nil()
		assert.That(t, string(b)).Equal("bool=true&float=3.14&int=123&string=hello")
	})

	t.Run("special characters", func(t *testing.T) {
		m := map[string]any{
			"message": "hello world!",
			"email":   "test@example.com",
		}
		b, err := FORM.Encode(m)
		assert.That(t, err).Nil()
		assert.That(t, string(b)).Equal("email=test%40example.com&message=hello+world%21")
	})

	t.Run("complex data", func(t *testing.T) {
		m := map[string]any{
			"name":     "张三",
			"age":      25,
			"active":   false,
			"balance":  123.45,
			"homepage": "https://example.com",
		}
		b, err := FORM.Encode(m)
		assert.That(t, err).Nil()
		assert.That(t, string(b)).Equal("active=false&age=25&balance=123.45&homepage=https%3A%2F%2Fexample.com&name=%E5%BC%A0%E4%B8%89")
	})

	t.Run("invalid input for JSON encoding", func(t *testing.T) {
		type Circular struct {
			Value int
			Ref   *Circular
		}
		c := &Circular{Value: 1}
		c.Ref = c
		_, err := FORM.Encode(c)
		assert.Error(t, err).String("json: unsupported value: encountered a cycle via *httputil.Circular")
	})
}

func TestJSONProtocol(t *testing.T) {
	m := map[string]any{
		"a": float64(1),
		"b": "2",
		"c": true,
	}

	b, err := JSON.Encode(m)
	assert.That(t, err).Nil()
	assert.That(t, string(b)).Equal(`{"a":1,"b":"2","c":true}`)

	var s interface{}
	err = JSON.Decode(b, &s)
	assert.That(t, err).Nil()
	assert.That(t, s).Equal(m)
}
