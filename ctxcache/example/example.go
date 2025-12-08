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

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/lvan100/golib/ctxcache"
)

// Example demonstrates basic usage of the cache
func Example() {
	// Step 1: Initialize cache in context
	ctx, cancel := ctxcache.Init(context.Background())
	defer cancel()

	// Step 2: Declare keys with their types
	userIDKey := ctxcache.Key[int]("user_id")
	userNameKey := ctxcache.Key[string]("user_name")
	permissionsKey := ctxcache.Key[[]string]("permissions")

	// Declare all keys at the beginning
	ctxcache.Declare(ctx, userIDKey)
	ctxcache.Declare(ctx, userNameKey)
	ctxcache.Declare(ctx, permissionsKey)

	// Step 3: Set values
	if err := ctxcache.Set(ctx, userIDKey, 12345); err != nil {
		fmt.Printf("Error setting user ID: %v\n", err)
		return
	}

	if err := ctxcache.Set(ctx, userNameKey, "Alice"); err != nil {
		fmt.Printf("Error setting user name: %v\n", err)
		return
	}

	if err := ctxcache.Set(ctx, permissionsKey, []string{"read", "write"}); err != nil {
		fmt.Printf("Error setting permissions: %v\n", err)
		return
	}

	// Step 4: Get values
	if userID, err := ctxcache.Get(ctx, userIDKey); err != nil {
		fmt.Printf("Error getting user ID: %v\n", err)
	} else {
		fmt.Printf("User ID: %d\n", userID)
	}

	if userName, err := ctxcache.Get(ctx, userNameKey); err != nil {
		fmt.Printf("Error getting user name: %v\n", err)
	} else {
		fmt.Printf("User Name: %s\n", userName)
	}

	if permissions, err := ctxcache.Get(ctx, permissionsKey); err != nil {
		fmt.Printf("Error getting permissions: %v\n", err)
	} else {
		fmt.Printf("Permissions: %v\n", permissions)
	}

	// Output:
	// User ID: 12345
	// User Name: Alice
	// Permissions: [read write]
}

// ExampleKeepAlive demonstrates how to use KeepAlive for background goroutines
func ExampleKeepAlive() {
	// Initialize cache
	ctx, cancel := ctxcache.Init(context.Background())
	defer cancel()

	// Declare a key
	taskIDKey := ctxcache.Key[string]("task_id")
	ctxcache.Declare(ctx, taskIDKey)

	// Set a value
	if err := ctxcache.Set(ctx, taskIDKey, "task-12345"); err != nil {
		fmt.Printf("Error setting task ID: %v\n", err)
		return
	}

	// Mark this key for keep alive - value will survive cache cleanup
	ctxcache.KeepAlive(ctx, taskIDKey)

	// Channel to signal completion of goroutine
	done := make(chan bool)

	// Simulate background goroutine that needs this value after request ends
	go func() {
		defer func() { done <- true }()

		// Simulate some work
		time.Sleep(100 * time.Millisecond)

		// Even after cancel() is called, we can still access the value
		// because we marked it with KeepAlive
		if taskID, err := ctxcache.Get(ctx, taskIDKey); err != nil {
			fmt.Printf("Background task error: %v\n", err)
		} else {
			fmt.Printf("Background task ID: %s\n", taskID)
		}
	}()

	// Give the goroutine a chance to start
	time.Sleep(200 * time.Millisecond)

	// Cancel the cache (simulating end of HTTP request)
	cancel()

	// Wait for background goroutine to complete
	<-done

	// Output:
	// Background task ID: task-12345
}

// ExampleErrors demonstrates error handling
func ExampleErrors() {
	ctx := context.Background() // No cache initialized

	// Try to use a key without initializing cache
	userIDKey := ctxcache.Key[int]("user_id")

	if _, err := ctxcache.Get(ctx, userIDKey); err != nil {
		fmt.Printf("Expected error: %v\n", err)
	}

	// Initialize cache properly
	ctx, cancel := ctxcache.Init(ctx)
	defer cancel()

	// Try to use undeclared key
	if _, err := ctxcache.Get(ctx, userIDKey); err != nil {
		fmt.Printf("Expected error: %v\n", err)
	}

	// Declare and use key properly
	ctxcache.Declare(ctx, userIDKey)

	// Try to get unset value
	if _, err := ctxcache.Get(ctx, userIDKey); err != nil {
		fmt.Printf("Expected error: %v\n", err)
	}

	// Set and get value
	if err := ctxcache.Set(ctx, userIDKey, 42); err != nil {
		fmt.Printf("Error setting user ID: %v\n", err)
		return
	}

	if userID, err := ctxcache.Get(ctx, userIDKey); err != nil {
		fmt.Printf("Unexpected error: %v\n", err)
	} else {
		fmt.Printf("User ID: %d\n", userID)
	}

	// Try to set value again (should fail)
	if err := ctxcache.Set(ctx, userIDKey, 43); err != nil {
		fmt.Printf("Expected error: %v\n", err)
	}

	// Output:
	// Expected error: user_id(int): cache not initialized
	// Expected error: user_id(int): key not declared
	// Expected error: user_id(int): value not set
	// User ID: 42
	// Expected error: user_id(int): value already set
}

func main() {
	fmt.Println("=== Basic Cache Usage ===")
	Example()

	fmt.Println("\n=== Keep Alive Feature ===")
	ExampleKeepAlive()

	fmt.Println("\n=== Error Handling ===")
	ExampleErrors()
}
