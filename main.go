package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

func countWords(ctx context.Context, filePath string, wg *sync.WaitGroup, wordCounts chan<- map[string]int) {
	// Sleep for 11 seconds to simulate context cancellation. Only use sleep for testing
	// time.Sleep(11 * time.Second)
	defer wg.Done()

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Failed to open file %s: %v", filePath, err)
		return
	}
	defer file.Close()

	wordCount := make(map[string]int)
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanWords)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			log.Printf("Processing canceled for file %s", filePath)
			return
		default:
			word := strings.ToLower(scanner.Text())
			wordCount[word]++
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading file %s: %v", filePath, err)
	}

	wordCounts <- wordCount
}

func aggregateCounts(ctx context.Context, wordCounts <-chan map[string]int, finalCounts chan<- map[string]int, done chan<- struct{}) {
	combinedCounts := make(map[string]int)

	for {
		select {
		case <-ctx.Done():
			log.Println("Aggregate canceled")
			return
		case wordCount, ok := <-wordCounts:
			if !ok {
				finalCounts <- combinedCounts
				done <- struct{}{}
				return
			}
			for word, count := range wordCount {
				combinedCounts[word] += count
			}
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Please provide the file paths as command-line arguments")
	}

	filePaths := os.Args[1:]

	var wg sync.WaitGroup
	wordCounts := make(chan map[string]int, len(filePaths))
	finalCounts := make(chan map[string]int, 1)
	done := make(chan struct{})

	// Cancel if it takes longer than 10 secs
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, filePath := range filePaths {
		wg.Add(1)
		go countWords(ctx, filePath, &wg, wordCounts)
	}

	go func() {
		wg.Wait()
		close(wordCounts)
	}()

	go aggregateCounts(ctx, wordCounts, finalCounts, done)

	select {
	case <-done:
		combinedCounts := <-finalCounts
		for word, count := range combinedCounts {
			fmt.Printf("%s: %d\n", word, count)
		}
	case <-ctx.Done():
		log.Println("Operation timed out or was canceled")
	}
}
