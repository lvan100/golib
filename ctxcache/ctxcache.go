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

// Package ctxcache provides a strongly-typed, context-scoped cache for
// request- and goroutine-lifecycle data.
//
// ctxcache attaches a concurrency-safe key–value store to a context.Context,
// allowing data to be implicitly propagated across call boundaries without
// polluting function signatures.
//
// Keys are declared explicitly and bound to a concrete Go type using generics,
// ensuring type safety and preventing key collisions. Each key may be assigned
// at most once, making ctxcache suitable for passing derived or request-scoped
// data such as authenticated users, permissions, trace metadata, or computed
// intermediates.
//
// The cache lifecycle is tied to the context via an explicit cancel function.
// By default, all cached values are cleared when the cancel function is called.
// For background goroutines that outlive the original request, selected values
// can be preserved using KeepAlive.
//
// Typical usage:
//
//  1. Initialize the cache at the request boundary (e.g. HTTP middleware).
//  2. Declare required keys at the entry point of business logic.
//  3. Set each value exactly once and retrieve it via Get.
//  4. Defer the cancel function to clean up request-scoped data.
//
// ctxcache is not a general-purpose cache. It is designed for structured,
// short-lived, in-process data bound to a context's lifetime.
package ctxcache

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrCacheNotInitialized = errors.New("cache not initialized")
	ErrCacheAlreadyCleared = errors.New("cache already cleared")
	ErrKeyNotDeclared      = errors.New("key not declared")
	ErrKeyTypeMismatch     = errors.New("key type mismatch")
	ErrValueNotSet         = errors.New("value not set")
	ErrValueAlreadySet     = errors.New("value already set")
)

type cacheKeyType struct{}

var cacheKey = cacheKeyType{}

// Cache holds user-scoped data associated with a context.
//
// Internally, Cache attaches a concurrency-safe map to a context.Context.
// The map is propagated through the context without appearing in function
// signatures, making it suitable for passing data across modules.
//
// Typical usage:
//
//  1. Initialize the cache in HTTP middleware using Init.
//  2. Declare all required keys at the entry point of your business logic using Declare.
//  3. Set values using Set and retrieve them using Get.
//  4. Defer the cancel function returned by Init to clean up cached data.
//
// By default, all cached values are removed when the cancel function runs.
// KeepAlive can be used to extend the lifetime of specific keys for background goroutines.
type Cache struct {
	mutex     sync.Mutex
	values    map[any]any
	keepAlive map[any]struct{}
	cleared   bool
}

// Clear removes all cached values and marks the cache as cleared.
func (cache *Cache) Clear() {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	if cache.cleared {
		return
	}
	cache.cleared = true

	for k := range cache.values {
		if _, ok := cache.keepAlive[k]; !ok {
			delete(cache.values, k)
		}
	}

	clear(cache.keepAlive)
}

// getCache retrieves the Cache attached to the given context, if any.
func getCache(ctx context.Context) (*Cache, bool) {
	cache, ok := ctx.Value(&cacheKey).(*Cache)
	return cache, ok
}

// Init attaches a Cache to the given context and returns the new context
// along with a cancel function.
//
// Only one Cache may be attached to a context. Repeated calls to Init
// are safe: if a Cache already exists, Init returns the original context
// and the same cancel function created previously.
//
// Only the first call to the cancel function performs actual cleanup.
// Subsequent calls are safe but have no effect.
//
// Typical usage is in HTTP middleware, where the cancel function is deferred
// to clean up cached data at the end of the request.
//
// Background goroutines:
//
// If a goroutine continues executing after the request completes and
// still uses the same context, cached data may outlive the request.
// In such cases, KeepAlive should be used to explicitly extend the lifetime
// of specific values.
//
// If the cancel function is never called, cached data will remain reachable
// as long as the associated context is reachable.
func Init(ctx context.Context) (_ context.Context, cancel func()) {
	if cache, ok := getCache(ctx); ok {
		return ctx, cache.Clear
	}

	m := &Cache{
		values:    make(map[any]any),
		keepAlive: make(map[any]struct{}),
	}

	return context.WithValue(ctx, &cacheKey, m), m.Clear
}

// KeepAlive prevents a key from being removed during cache cleanup.
//
// Use this when launching background goroutines that may continue executing
// after an HTTP request completes but still require access to cached data.
//
// Keys marked with KeepAlive will not be deleted when the cancel function
// returned by Init is executed.
//
// The cached value remains stored in the Cache as long as the Cache itself
// is reachable (typically via the associated context). Once the Cache
// becomes unreachable, values are reclaimed by garbage collection.
//
// KeepAlive only affects cleanup triggered by the cancel function; it does
// not extend the lifetime of the context itself.
//
// If the Cache is not initialized or has already been cleared, the call
// is silently ignored.
func KeepAlive[T any](ctx context.Context, k TypeKey[T]) {
	cache, ok := getCache(ctx)
	if !ok {
		return
	}

	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	if cache.cleared {
		return
	}

	cache.keepAlive[k] = struct{}{}
}

// TypeValue wraps a cached value.
//
// IsSet indicates whether the value has been set.
// Value holds the actual data.
type TypeValue[T any] struct {
	IsSet bool
	Value T
}

// TypeKey represents a strongly typed cache key.
//
// Each key is associated with a specific Go type via generics, preventing
// collisions between keys of different types—even if they share the same
// string identifier.
type TypeKey[T any] struct {
	Key string
}

// Key creates a new TypeKey with the given string identifier.
func Key[T any](key string) TypeKey[T] {
	return TypeKey[T]{Key: key}
}

func (k TypeKey[T]) String() string {
	var zero T
	return fmt.Sprintf("%s(%T)", k.Key, zero)
}

// Declare registers one or more keys in the Cache.
//
// Keys must be declared before they can be set or retrieved.
//
// If the Cache is not initialized or already cleared, Declare is a no-op.
//
// Declaring the same key multiple times is allowed; initialization is
// idempotent. However, Set must not be called before the first declaration.
//
// Best practice is to declare all required keys in a single place—typically
// at the beginning of an HTTP handler or controller—rather than in middleware.
func Declare[T any](ctx context.Context, keys ...TypeKey[T]) {
	cache, ok := getCache(ctx)
	if !ok {
		return
	}

	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	if cache.cleared {
		return
	}

	for _, k := range keys {
		if _, exists := cache.values[k]; !exists {
			cache.values[k] = TypeValue[T]{IsSet: false}
		}
	}
}

// Get retrieves the value associated with the given key.
//
// The key must have been declared using Declare, and a value must have
// been assigned using Set.
//
// Returns an error if:
// - the cache is not initialized or has been cleared,
// - the key was not declared,
// - the value has not been set, or
// - the key's type does not match.
func Get[T any](ctx context.Context, k TypeKey[T]) (T, error) {
	var zero T

	cache, ok := getCache(ctx)
	if !ok {
		return zero, fmt.Errorf("%s: %w", k, ErrCacheNotInitialized)
	}

	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	if cache.cleared {
		return zero, fmt.Errorf("%s: %w", k, ErrCacheAlreadyCleared)
	}

	v, ok := cache.values[k]
	if !ok {
		return zero, fmt.Errorf("%s: %w", k, ErrKeyNotDeclared)
	}

	x, ok := v.(TypeValue[T])
	if !ok {
		return zero, fmt.Errorf("%s: %w", k, ErrKeyTypeMismatch)
	}

	if !x.IsSet {
		return zero, fmt.Errorf("%s: %w", k, ErrValueNotSet)
	}

	return x.Value, nil
}

// Set assigns a value to the given key.
//
// The key must have been declared using Declare.
// Each key may be assigned exactly once; subsequent attempts return an error.
//
// Returns an error if:
// - the cache is not initialized or has been cleared,
// - the key was not declared,
// - the value has already been set, or
// - the key's type does not match.
func Set[T any](ctx context.Context, k TypeKey[T], value T) error {
	cache, ok := getCache(ctx)
	if !ok {
		return fmt.Errorf("%s: %w", k, ErrCacheNotInitialized)
	}

	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	if cache.cleared {
		return fmt.Errorf("%s: %w", k, ErrCacheAlreadyCleared)
	}

	v, ok := cache.values[k]
	if !ok {
		return fmt.Errorf("%s: %w", k, ErrKeyNotDeclared)
	}

	x, ok := v.(TypeValue[T])
	if !ok {
		return fmt.Errorf("%s: %w", k, ErrKeyTypeMismatch)
	}

	if x.IsSet {
		return fmt.Errorf("%s: %w", k, ErrValueAlreadySet)
	}

	cache.values[k] = TypeValue[T]{IsSet: true, Value: value}
	return nil
}
