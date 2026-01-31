package main

import (
    "fmt"
    "sync"
    "time"
)

type SafeMap struct {
    mu sync.Mutex
    m  map[int]int
}

func main() {
    runs := 3
    results := make([]time.Duration, runs)
    
    for run := 0; run < runs; run++ {
        fmt.Printf("\n=== Run %d ===\n", run+1)
        
        safeMap := SafeMap{
            m: make(map[int]int),
        }
        
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
                    
                    safeMap.mu.Lock()
                    safeMap.m[key] = i
                    safeMap.mu.Unlock()
                }
            }(g)
        }
        
        wg.Wait()
        duration := time.Since(start)
        results[run] = duration
        
        fmt.Printf("Map length: %d\n", len(safeMap.m))
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