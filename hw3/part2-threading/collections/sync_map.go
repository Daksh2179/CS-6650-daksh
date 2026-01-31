package main

import (
    "fmt"
    "sync"
    "time"
)

func main() {
    runs := 3
    results := make([]time.Duration, runs)
    
    for run := 0; run < runs; run++ {
        fmt.Printf("\n=== Run %d ===\n", run+1)
        
        var m sync.Map
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
                    m.Store(key, i)
                }
            }(g)
        }
        
        wg.Wait()
        duration := time.Since(start)
        results[run] = duration
        
        // Count entries
        count := 0
        m.Range(func(key, value interface{}) bool {
            count++
            return true
        })
        
        fmt.Printf("Map length: %d\n", count)
        fmt.Printf("Time taken: %v\n", duration)
    }
    
    // Calculate mean
    var total time.Duration
    for _, d := range results {
        total += d
    }
    mean := total / time.Duration(len(results))
    fmt.Printf("\n=== Mean time: %v ===\n", mean)
}