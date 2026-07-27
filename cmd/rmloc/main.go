package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"rmloc/internal/app"
	"rmloc/internal/app/rmmz"
	"rmloc/internal/ollama"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(
			os.Stderr,
			"Использование: %s <каталог игры>\n",
			os.Args[0],
		)
		os.Exit(1)
	}

	gameDir, err := filepath.Abs(os.Args[1])
	if err != nil {
		log.Fatalf(
			"Не удалось определить путь к игре: %v",
			err,
		)
	}

	ollamaClient := ollama.NewClient(
		"http://localhost:11434",
		"qwen3:4b",
	)

	cachePath := filepath.Join(
		gameDir,
		".rmloc",
		"translation-cache.jsonl",
	)

	if err := ollamaClient.EnablePersistentCache(
		cachePath,
	); err != nil {
		log.Fatalf(
			"Не удалось загрузить кэш переводов: %v",
			err,
		)
	}

	fmt.Printf("Каталог игры: %s\n", gameDir)
	fmt.Printf("Кэш переводов: %s\n\n", cachePath)

	rmmzEngine := rmmz.NewRmmzEngine(ollamaClient)
	application := app.NewApp(*rmmzEngine, 0)

	runErr := application.Run(gameDir)
	ollamaClient.PrintMetrics(os.Stdout)

	if runErr != nil {
		log.Printf("Перевод завершён с ошибкой: %v", runErr)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("Перевод успешно завершён.")
}
