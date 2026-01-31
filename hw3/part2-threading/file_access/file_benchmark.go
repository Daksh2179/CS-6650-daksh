package main

import (
    "bufio"
    "fmt"
    "os"
    "time"
)

func unbufferedWrite(filename string, iterations int) time.Duration {
    f, err := os.Create(filename)
    if err != nil {
        panic(err)
    }
    defer f.Close()
    
    start := time.Now()
    
    for i := 0; i < iterations; i++ {
        line := fmt.Sprintf("Line %d: This is some test data\n", i)
        _, err := f.Write([]byte(line))
        if err != nil {
            panic(err)
        }
    }
    
    return time.Since(start)
}

func bufferedWrite(filename string, iterations int) time.Duration {
    f, err := os.Create(filename)
    if err != nil {
        panic(err)
    }
    defer f.Close()
    
    writer := bufio.NewWriter(f)
    
    start := time.Now()
    
    for i := 0; i < iterations; i++ {
        line := fmt.Sprintf("Line %d: This is some test data\n", i)
        _, err := writer.WriteString(line)
        if err != nil {
            panic(err)
        }
    }
    
    writer.Flush()
    
    return time.Since(start)
}

func main() {
    iterations := 100000
    
    fmt.Println("Testing file write performance...")
    fmt.Printf("Writing %d lines\n\n", iterations)
    
    unbufferedTime := unbufferedWrite("unbuffered.txt", iterations)
    fmt.Printf("Unbuffered write: %v\n", unbufferedTime)
    
    bufferedTime := bufferedWrite("buffered.txt", iterations)
    fmt.Printf("Buffered write: %v\n", bufferedTime)
    
    speedup := float64(unbufferedTime) / float64(bufferedTime)
    fmt.Printf("\nBuffered is %.2fx faster\n", speedup)
    
    // Cleanup
    os.Remove("unbuffered.txt")
    os.Remove("buffered.txt")
}