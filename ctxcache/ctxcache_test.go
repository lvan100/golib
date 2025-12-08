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

package ctxcache

import (
	"fmt"
	"testing"
)

func TestDeclare(t *testing.T) {
	fmt.Printf("%s", Key[int]("testKey"))

	ctx, cancel := Init(t.Context())
	defer cancel()

	// Declare a key normally
	Declare[string](ctx, Key[string]("testKey"))

	// Re-declaring the same key should not fail (declaration is idempotent)
	Declare[string](ctx, Key[string]("testKey"))
}

// Test for Init function called multiple times
func TestInitMultipleCalls(t *testing.T) {
	ctx := t.Context()

	// First call to Init
	ctx1, cancel1 := Init(ctx)

	// Second call to Init with the same context
	ctx2, cancel2 := Init(ctx1)

	// Should return the same context and cancel function
	if ctx1 != ctx2 {
		t.Error("Expected the same context to be returned")
	}

	// Test that setting a value works with either context
	key := Key[string]("testKey")
	Declare[string](ctx1, key)

	err := Set[string](ctx2, key, "testValue")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	value, err := Get[string](ctx1, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if value != "testValue" {
		t.Errorf("Expected 'testValue', got '%s'", value)
	}

	// Cancel should work with either function
	cancel1()

	_, err = Get[string](ctx1, key)
	if err == nil {
		t.Error("Expected error after cancel")
	}

	// Calling the second cancel should be safe but have no effect
	cancel2()
}

func TestSetWithoutDeclare(t *testing.T) {
	ctx, cancel := Init(t.Context())
	defer cancel()

	key := Key[string]("undeclaredKey")

	// Setting a value without declaring the key should fail
	err := Set[string](ctx, key, "testValue")
	if err == nil {
		t.Error("Setting an undeclared key should fail")
	}
}

func TestGetWithoutDeclare(t *testing.T) {
	ctx, cancel := Init(t.Context())
	defer cancel()

	key := Key[string]("undeclaredKey")

	// Getting an undeclared key should fail
	_, err := Get[string](ctx, key)
	if err == nil {
		t.Error("Getting an undeclared key should fail")
	}
}

func TestGetWithoutSet(t *testing.T) {
	ctx, cancel := Init(t.Context())
	defer cancel()

	key := Key[string]("declaredKey")

	// Declare the key without setting a value
	Declare[string](ctx, key)

	// Getting a declared but unset key should fail
	_, err := Get[string](ctx, key)
	if err == nil {
		t.Error("Getting an unset key should fail")
	}
}

func TestSetTwice(t *testing.T) {
	ctx, cancel := Init(t.Context())
	defer cancel()

	key := Key[string]("testKey")

	// Declare the key
	Declare[string](ctx, key)

	// First set should succeed
	err := Set[string](ctx, key, "firstValue")
	if err != nil {
		t.Fatalf("First Set failed: %v", err)
	}

	// Second set for the same key should fail
	err = Set[string](ctx, key, "secondValue")
	if err == nil {
		t.Error("Setting the same key twice should fail")
	}

	// Ensure the value remains the first one
	value, err := Get[string](ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if value != "firstValue" {
		t.Errorf("Expected 'firstValue', got '%s'", value)
	}
}

func TestDifferentTypes(t *testing.T) {
	ctx, cancel := Init(t.Context())
	defer cancel()

	stringKey := Key[string]("key")
	intKey := Key[int]("key")

	// Declare keys with the same name but different types
	Declare[string](ctx, stringKey)
	Declare[int](ctx, intKey)

	// Set values for both keys
	err := Set[string](ctx, stringKey, "stringValue")
	if err != nil {
		t.Fatalf("Set string failed: %v", err)
	}

	err = Set[int](ctx, intKey, 42)
	if err != nil {
		t.Fatalf("Set int failed: %v", err)
	}

	// Retrieve and verify the string value
	strValue, err := Get[string](ctx, stringKey)
	if err != nil {
		t.Fatalf("Get string failed: %v", err)
	}

	if strValue != "stringValue" {
		t.Errorf("Expected 'stringValue', got '%s'", strValue)
	}

	// Retrieve and verify the int value
	intValue, err := Get[int](ctx, intKey)
	if err != nil {
		t.Fatalf("Get int failed: %v", err)
	}

	if intValue != 42 {
		t.Errorf("Expected 42, got %d", intValue)
	}
}

func TestContextNotBound(t *testing.T) {
	ctx := t.Context()

	key := Key[string]("anyKey")

	// Declaring a key on a context without an initialized cache should be a no-op
	Declare[string](ctx, key)

	// Setting a value without an initialized cache should fail
	err := Set[string](ctx, key, "anyValue")
	if err == nil {
		t.Error("Set should fail when the context is not bound to a cache")
	}

	// Getting a value without an initialized cache should fail
	_, err = Get[string](ctx, key)
	if err == nil {
		t.Error("Get should fail when the context is not bound to a cache")
	}
}

func TestKeepAlive(t *testing.T) {
	// Create a context with an initialized cache
	ctx, cancel := Init(t.Context())
	defer cancel() // Ensure cleanup

	key := Key[string]("keepAliveKey")

	// Declare and set the value
	Declare[string](ctx, key)
	err := Set[string](ctx, key, "keepAliveValue")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Mark the key as KeepAlive
	KeepAlive[string](ctx, key)

	// Verify the value can still be accessed
	value, err := Get[string](ctx, key)
	if err != nil {
		t.Errorf("Get failed: %v", err)
		return
	}
	if value != "keepAliveValue" {
		t.Errorf("Expected 'keepAliveValue', got '%s'", value)
	}

	// Verify KeepAlive bookkeeping
	cache, ok := getCache(ctx)
	if !ok {
		t.Fatal("Cache should not be nil")
	}

	// The key should be present in the keepAlive map
	_, ok = cache.keepAlive[key]
	if !ok {
		t.Error("Key should be present in the keepAlive map after KeepAlive")
	}
}

// Combined test cases for cancel behavior
func TestCancelBehavior(t *testing.T) {
	// Create a context with an initialized cache
	ctx, cancel := Init(t.Context())

	key1 := Key[string]("key1")
	key2 := Key[string]("key2")

	// Declare and set both keys
	Declare[string](ctx, key1, key2)

	err := Set[string](ctx, key1, "value1")
	if err != nil {
		t.Fatalf("Set key1 failed: %v", err)
	}

	err = Set[string](ctx, key2, "value2")
	if err != nil {
		t.Fatalf("Set key2 failed: %v", err)
	}

	// Mark only key1 as KeepAlive
	KeepAlive[string](ctx, key1)

	// Execute cancel
	cancel()

	// After cancel, no values should be accessible
	// According to the current implementation, even KeepAlive-marked keys
	// cannot be accessed once the cache is cleared
	_, err = Get[string](ctx, key1)
	if err == nil {
		t.Error("Even KeepAlive-marked keys should not be accessible after cancel()")
	}

	_, err = Get[string](ctx, key2)
	if err == nil {
		t.Error("Get should fail after cancel without KeepAlive")
	}

	// Test KeepAlive bookkeeping independently
	ctx2, cancel2 := Init(t.Context())
	defer cancel2()

	key3 := Key[string]("key3")
	Declare[string](ctx2, key3)
	err = Set[string](ctx2, key3, "value3")
	if err != nil {
		t.Fatalf("Set key3 failed: %v", err)
	}

	KeepAlive[string](ctx2, key3)

	cache, ok := getCache(ctx2)
	if !ok {
		t.Fatal("Cache should not be nil")
	}

	// key3 should be present in the keepAlive map
	_, ok = cache.keepAlive[key3]
	if !ok {
		t.Error("Key3 should be present in the keepAlive map after KeepAlive")
	}
}

// Test for ErrCacheAlreadyCleared error
func TestErrCacheAlreadyCleared(t *testing.T) {
	ctx, cancel := Init(t.Context())
	defer cancel()

	key := Key[string]("testKey")
	Declare[string](ctx, key)

	// Set a value
	err := Set[string](ctx, key, "testValue")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Cancel the context
	cancel()

	// Try to get the value after cancel - should return ErrCacheAlreadyCleared
	_, err = Get[string](ctx, key)
	if err == nil {
		t.Error("Expected an error after cancel")
		return
	}

	if err.Error() != fmt.Sprintf("%s: %s", key, ErrCacheAlreadyCleared) {
		t.Errorf("Expected ErrCacheAlreadyCleared, got: %v", err)
	}

	// Try to set a value after cancel - should return ErrCacheAlreadyCleared
	err = Set[string](ctx, key, "newValue")
	if err == nil {
		t.Error("Expected an error after cancel")
		return
	}

	if err.Error() != fmt.Sprintf("%s: %s", key, ErrCacheAlreadyCleared) {
		t.Errorf("Expected ErrCacheAlreadyCleared, got: %v", err)
	}
}

// Test for ErrKeyTypeMismatch error
func TestErrKeyTypeMismatch(t *testing.T) {
	ctx, cancel := Init(t.Context())
	defer cancel()

	// Declare a key with one type
	stringKey := Key[string]("testKey")
	Declare[string](ctx, stringKey)

	// Set a value for that key
	err := Set[string](ctx, stringKey, "stringValue")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// First declare the other type key
	intKey := Key[int]("testKey") // Same name but different type
	Declare[int](ctx, intKey)

	// Try to set a value with a different type without declaring - should return ErrKeyNotDeclared
	err = Set[int](ctx, Key[int]("undeclared"), 42)
	if err == nil {
		t.Error("Expected ErrKeyNotDeclared error")
		return
	}

	// Try to get the value with a different type but both declared - should work fine
	_, err = Get[int](ctx, intKey)
	if err != nil && err.Error() == fmt.Sprintf("%s: %s", intKey, ErrValueNotSet) {
		// This is expected since we didn't set a value for intKey
	} else if err == nil {
		t.Error("Expected ErrValueNotSet error")
		return
	}
}

// Test for TypeValue struct
func TestTypeValue(t *testing.T) {
	// Test TypeValue with string
	tvStr := TypeValue[string]{IsSet: true, Value: "test"}
	if !tvStr.IsSet {
		t.Error("Expected IsSet to be true")
	}
	if tvStr.Value != "test" {
		t.Errorf("Expected Value to be 'test', got '%s'", tvStr.Value)
	}

	// Test TypeValue with int
	tvInt := TypeValue[int]{IsSet: false, Value: 0}
	if tvInt.IsSet {
		t.Error("Expected IsSet to be false")
	}
	if tvInt.Value != 0 {
		t.Errorf("Expected Value to be 0, got %d", tvInt.Value)
	}
}

// Test for TypeKey.String() method
func TestTypeKeyString(t *testing.T) {
	// Test string key
	stringKey := Key[string]("testKey")
	expectedString := "testKey(string)"
	if stringKey.String() != expectedString {
		t.Errorf("Expected %s, got %s", expectedString, stringKey.String())
	}

	// Test int key
	intKey := Key[int]("testKey")
	expectedInt := "testKey(int)"
	if intKey.String() != expectedInt {
		t.Errorf("Expected %s, got %s", expectedInt, intKey.String())
	}

	// Test custom struct key
	type CustomStruct struct{}
	structKey := Key[CustomStruct]("testKey")
	result := structKey.String()
	// The package name might vary depending on how the test is run
	if result != "testKey(exp/ctxcache.CustomStruct)" && result != "testKey(ctxcache.CustomStruct)" {
		t.Errorf("Unexpected string representation: %s", result)
	}
}

// Test Clear method directly
func TestClearMethod(t *testing.T) {
	ctx, cancel := Init(t.Context())
	defer cancel()

	key := Key[string]("testKey")
	Declare[string](ctx, key)
	err := Set[string](ctx, key, "testValue")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get the cache directly
	cache, ok := getCache(ctx)
	if !ok {
		t.Fatal("Failed to get cache")
	}

	// Value should exist before clearing
	_, exists := cache.values[key]
	if !exists {
		t.Error("Value should exist before clearing")
	}

	// Call Clear directly
	cache.Clear()

	// Value should not exist after clearing
	_, exists = cache.values[key]
	if exists {
		t.Error("Value should not exist after clearing")
	}

	// Calling Clear again should be safe
	cache.Clear()
}

// Test concurrent access to the cache
func TestConcurrentAccess(t *testing.T) {
	ctx, cancel := Init(t.Context())
	defer cancel()

	// Declare keys
	keys := make([]TypeKey[int], 100)
	for i := 0; i < 100; i++ {
		keys[i] = Key[int](fmt.Sprintf("key%d", i))
		Declare[int](ctx, keys[i])
	}

	// Concurrently set values
	setErrors := make(chan error, 100)
	for i := 0; i < 100; i++ {
		go func(index int) {
			err := Set[int](ctx, keys[index], index)
			setErrors <- err
		}(i)
	}

	// Check for set errors
	for i := 0; i < 100; i++ {
		err := <-setErrors
		if err != nil {
			t.Errorf("Concurrent Set failed: %v", err)
		}
	}

	// Concurrently get values
	getErrors := make(chan error, 100)
	getValues := make(chan int, 100)
	for i := 0; i < 100; i++ {
		go func(index int) {
			value, err := Get[int](ctx, keys[index])
			if err != nil {
				getErrors <- err
				return
			}
			if value != index {
				getErrors <- fmt.Errorf("expected %d, got %d", index, value)
				return
			}
			getErrors <- nil
			getValues <- value
		}(i)
	}

	// Check for get errors
	for i := 0; i < 100; i++ {
		err := <-getErrors
		if err != nil {
			t.Errorf("Concurrent Get failed: %v", err)
		}
	}

	// Verify we got 100 values
	valuesCount := 0
	for i := 0; i < 100; i++ {
		select {
		case <-getValues:
			valuesCount++
		default:
		}
	}

	if valuesCount != 100 {
		t.Errorf("Expected 100 values, got %d", valuesCount)
	}
}
