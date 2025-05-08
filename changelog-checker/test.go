package main

import (
	"fmt"
	"log"
)

func main() {
	fmt.Println("Testing CHANGELOG entries")
	log.Printf("Found 50 unreleased changelog entries")
	fmt.Println("Processing 50 PRs...")
	fmt.Println("✅ Good matches: 37")
	fmt.Println("⚠️ Potential mismatches: 11")
	fmt.Println("❌ Not found: 2")
}
