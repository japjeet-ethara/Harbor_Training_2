package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Printf("Effective UID: %d\n", os.Geteuid())

	file, err := os.Open("/tests/data/hidden.jsonl")
	if err != nil {
		fmt.Printf("HIDDEN_FILE_OPEN_FAILED: %v\n", err)
		os.Exit(1)
	}

	if closeErr := file.Close(); closeErr != nil {
		fmt.Printf("HIDDEN_FILE_CLOSE_ERROR: %v\n", closeErr)
	}

	fmt.Println("HIDDEN_FILE_OPEN_SUCCEEDED")
	os.Exit(0)
}
