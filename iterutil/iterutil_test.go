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

package iterutil

import (
	"testing"

	"github.com/lvan100/golib/testing/assert"
)

func fibonacci(n int) int {
	if n <= 0 {
		return 0
	} else if n == 1 {
		return 1
	} else {
		return fibonacci(n-1) + fibonacci(n-2)
	}
}

func BenchmarkRanges(b *testing.B) {
	const N = 5

	b.Run("for", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fibonacci(N)
		}
	})

	b.Run("loop", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			Times(5, func(i int) {
				fibonacci(N)
			})
		}
	})
}

func TestTimes(t *testing.T) {
	var arr []int
	Times(5, func(i int) {
		arr = append(arr, i)
	})
	assert.That(t, arr).Equal([]int{0, 1, 2, 3, 4})
}

func TestRanges(t *testing.T) {
	var arr []int
	Ranges(1, 5, func(i int) {
		arr = append(arr, i)
	})
	assert.That(t, arr).Equal([]int{1, 2, 3, 4})
	arr = nil
	Ranges(5, 1, func(i int) {
		arr = append(arr, i)
	})
	assert.That(t, arr).Equal([]int{5, 4, 3, 2})
}

func TestStepRanges(t *testing.T) {
	var arr []int
	StepRanges(1, 5, 2, func(i int) {
		arr = append(arr, i)
	})
	assert.That(t, arr).Equal([]int{1, 3})
	arr = nil
	StepRanges(5, 1, -2, func(i int) {
		arr = append(arr, i)
	})
	assert.That(t, arr).Equal([]int{5, 3})
}
