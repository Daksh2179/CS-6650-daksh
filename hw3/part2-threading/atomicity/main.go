package main

import (
    "fmt"
    "sync"
    "sync/atomic"
    "time"
)

func main() {
    // Non-atomic counter
    var normalCounter uint64 = 0
    var wg sync.WaitGroup

    // Atomic counter
    var atomicCounter uint64 = 0

    numGoroutines := 50
    incrementsPerGoroutine := 1000

    // Test with normal counter
    start := time.Now()
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < incrementsPerGoroutine; j++ {
                normalCounter++ // NOT thread-safe
            }
        }()
    }
    wg.Wait()
    normalDuration := time.Since(start)

	// Reset wait group for second test
    wg = sync.WaitGroup{}


    // Test with atomic counter
    start = time.Now()
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < incrementsPerGoroutine; j++ {
                atomic.AddUint64(&atomicCounter, 1) // Thread-safe
            }
        }()
    }
    wg.Wait()
    atomicDuration := time.Since(start)

    expectedValue := uint64(numGoroutines * incrementsPerGoroutine)
    fmt.Printf("Expected value: %d\n", expectedValue)
    fmt.Printf("Normal counter: %d (took %v)\n", normalCounter, normalDuration)
    fmt.Printf("Atomic counter: %d (took %v)\n", atomicCounter, atomicDuration)
}