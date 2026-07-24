package main

import (
	"fmt"
	"log"
	"os"
	"rmloc/internal/app"
	"rmloc/internal/app/rmmz"
	"rmloc/internal/ollama"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(
			os.Stderr, "Использование: %s <каталог игры>\n", os.Args[0],
		)
		os.Exit(1)
	}

	ollamaClient := ollama.NewClient(
		"http://localhost:11434",
		"qwen3:4b",
	)

	rmmzEngine := rmmz.NewRmmzEngine(ollamaClient)
	application := app.NewApp(*rmmzEngine, 0)
	if err := application.Run(os.Args[1]); err != nil {
		log.Fatal(err)
	}
}
