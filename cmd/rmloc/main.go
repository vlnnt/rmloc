package main

import (
	"log"

	"rmloc/internal/tui"
)

func main() {
	if err := tui.Run(); err != nil {
		log.Fatalf("Failed to run rmloc: %v", err)
	}
}
