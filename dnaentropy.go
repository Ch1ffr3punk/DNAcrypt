package main

import (
    "fmt"
    "io/ioutil"
    "math"
    "os"
)

// Calculates the Shannon entropy of a DNA sequence
func shannonEntropy(dna string) float64 {
    counts := make(map[rune]float64)
    length := float64(len(dna))

    for _, base := range dna {
        counts[base]++
    }

    var entropy float64
    for _, count := range counts {
        prob := count / length
        entropy -= prob * math.Log2(prob)
    }

    return entropy
}

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: dna-entropy <keyfile>")
        os.Exit(1)
    }

    keyFile := os.Args[1]
    dnaBytes, err := ioutil.ReadFile(keyFile)
    if err != nil {
        fmt.Printf("Error reading file: %v\n", err)
        os.Exit(1)
    }

    dnaKey := string(dnaBytes)
    entropy := shannonEntropy(dnaKey)
    fmt.Printf("Shannon entropy of the DNA key (%d bases): %.4f\n", len(dnaKey), entropy)

    if entropy > 1.9 {
        fmt.Println("✔ The key has high entropy and appears well-distributed.")
    } else {
        fmt.Println("⚠ The key may contain patterns—further testing could be useful.")
    }
}
