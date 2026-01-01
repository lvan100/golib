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

package hashutil

import (
	"hash/fnv"
	"testing"

	"github.com/lvan100/golib/testing/assert"
)

func TestFNV1a64(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "hello", input: "hello"},
		{name: "world", input: "world"},
		{name: "Go-Spring", input: "Go-Spring"},
		{name: "fnv hash test", input: "fnv hash test"},
		{name: "numbers", input: "123456789"},
		{name: "lowercase letters", input: "abcdefghijklmnopqrstuvwxyz"},
		{name: "uppercase letters", input: "ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
		{name: "alphanumeric", input: "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"},
		{name: "special characters", input: "!@#$%^&*()_+-=[]{}|;':\",./<>?"},
		{name: "chinese characters", input: "测试中文字符串"},
		{name: "emojis", input: "🚀🌟💻🎉"},
		{name: "long string", input: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+-=[]{}|;':\",./<>?🚀🌟💻🎉テスト長字符串"},
		{name: "single character", input: "a"},
		{name: "two characters", input: "ab"},
		{name: "unicode characters", input: "汉字 and にほん"},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ourHash := FNV1a64(tt.input)

			h64 := fnv.New64a()
			_, _ = h64.Write([]byte(tt.input))
			stdHash := h64.Sum64()

			assert.That(t, ourHash).Equal(stdHash, tt.name)
		})
	}
}

func BenchmarkFNV1a64(b *testing.B) {
	testCases := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "short", input: "hello"},
		{name: "medium", input: "This is a medium length string for benchmarking FNV1a64 implementation"},
		{name: "long", input: "This is a long string for benchmarking FNV1a64 implementation. This is a long string for benchmarking FNV1a64 implementation. This is a long string for benchmarking FNV1a64 implementation."},
		{name: "very long", input: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#\\$%^&*()_+-=[]{}|;':\\\",./<>?🚀🌟💻🎉测试长字符串abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#\\$%^&*()_+-=[]{}|;':\\\",./<>?🚀🌟💻🎉テスト長字符串abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#\\$%^&*()_+-=[]{}|;':\\\",./<>?🚀🌟💻🎉テスト長字符串"},
	}

	for _, tt := range testCases {
		b.Run("Our_"+tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = FNV1a64(tt.input)
			}
		})

		b.Run("Std_"+tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				h64 := fnv.New64a()
				_, _ = h64.Write([]byte(tt.input))
				_ = h64.Sum64()
			}
		})
	}
}
