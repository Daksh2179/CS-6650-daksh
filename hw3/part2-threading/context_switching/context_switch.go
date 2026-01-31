package main

import (
    "fmt"
    "runtime"
    "time"
)

func pingPong(iterations int) time.Duration {
    ch1 := make(chan struct{})
    ch2 := make(chan struct{})
    
    start := time.Now()
    
    go func() {
        for i := 0; i < iterations; i++ {
            <-ch1
            ch2 <- struct{}{}
        }
    }()
    
    go func() {
        for i := 0; i < iterations; i++ {
            ch1 <- struct{}{}
            <-ch2
        }
    }()
    
    time.Sleep(100 * time.Millisecond) // Let goroutines finish
    
    return time.Since(start)
}

func main() {
    iterations := 1000000
    
    // Test with 1 OS thread
    fmt.Println("=== Single OS Thread (GOMAXPROCS=1) ===")
    runtime.GOMAXPROCS(1)
    
    singleThreadTime := pingPong(iterations)
    fmt.Printf("Total time: %v\n", singleThreadTime)
    avgSwitchTime := singleThreadTime / time.Duration(iterations*2)
    fmt.Printf("Average switch time: %v\n\n", avgSwitchTime)
    
    // Test with multiple OS threads
    fmt.Println("=== Multiple OS Threads (GOMAXPROCS=default) ===")
    runtime.GOMAXPROCS(runtime.NumCPU())
    
    multiThreadTime := pingPong(iterations)
    fmt.Printf("Total time: %v\n", multiThreadTime)
    avgSwitchTimeMulti := multiThreadTime / time.Duration(iterations*2)
    fmt.Printf("Average switch time: %v\n", avgSwitchTimeMulti)
    
    fmt.Printf("\nSpeedup: %.2fx\n", float64(singleThreadTime)/float64(multiThreadTime))
}