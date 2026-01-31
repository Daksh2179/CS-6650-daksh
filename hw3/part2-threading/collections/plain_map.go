package main

import (
    "fmt"
    "sync"
    "time"
)

func main() {
    runs := 3
    
    for run := 1; run <= runs; run++ {
        fmt.Printf("\n=== Run %d ===\n", run)
        
        m := make(map[int]int)
        var wg sync.WaitGroup
        numGoroutines := 50
        iterationsPerGoroutine := 1000
        
        start := time.Now()
        
        for g := 0; g < numGoroutines; g++ {
            wg.Add(1)
            go func(goroutineID int) {
                defer wg.Done()
                for i := 0; i < iterationsPerGoroutine; i++ {
                    key := goroutineID*1000 + i
                    m[key] = i  // This will cause race condition!
                }
            }(g)
        }
        
        wg.Wait()
        duration := time.Since(start)
        
        fmt.Printf("Map length: %d\n", len(m))
        fmt.Printf("Time taken: %v\n", duration)
        fmt.Printf("Expected length: %d\n", numGoroutines*iterationsPerGoroutine)
    }
}