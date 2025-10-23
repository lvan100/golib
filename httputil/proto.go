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
	"encoding/json"
	"net/url"
	"strconv"
)

var (
	// FORM provides an implementation of the Protocol interface using
	// application/x-www-form-urlencoded encoding.
	FORM Protocol = &FormProtocol{}

	// JSON provides an implementation of the Protocol interface using
	// JSON encoding.
	JSON Protocol = &JSONProtocol{}
)

// Protocol defines a common interface for encoding and decoding data
// into different serialization formats.
type Protocol interface {
	Encode(v any) ([]byte, error)
	Decode(data []byte, v any) error
}

// FormProtocol implements the Protocol interface for the
// application/x-www-form-urlencoded format.
type FormProtocol struct{}

// Encode serializes the given value into an application/x-www-form-urlencoded
// encoded byte slice.
func (p *FormProtocol) Encode(i any) ([]byte, error) {
	if i == nil {
		return []byte(""), nil
	}

	b, err := JSON.Encode(i)
	if err != nil {
		return nil, err
	}

	var m map[string]json.RawMessage
	if err = JSON.Decode(b, &m); err != nil {
		return nil, err
	}

	u := url.Values{}
	for k, v := range m {
		if len(v) > 0 && v[0] == '"' {
			s, err := strconv.Unquote(string(v))
			if err != nil {
				return nil, err
			}
			u.Set(k, s)
		} else {
			u.Set(k, string(v))
		}
	}

	return []byte(u.Encode()), nil
}

// Decode deserializes application/x-www-form-urlencoded data into the
// provided struct or map.
func (p *FormProtocol) Decode(data []byte, v any) error {
	panic("not implemented")
}

// JSONProtocol implements the Protocol interface using JSON encoding.
type JSONProtocol struct{}

// Encode serializes the given value into a JSON-encoded byte slice.
func (p *JSONProtocol) Encode(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Decode deserializes JSON data into the provided value.
func (p *JSONProtocol) Decode(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
