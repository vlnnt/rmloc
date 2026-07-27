package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"rmloc/internal/app"
	"rmloc/internal/app/rmmz"
	"rmloc/internal/ollama"
)

func main() {
	url := flag.String("url", "", "Ollama API URL, for example http://localhost:11434")
	model := flag.String("model", "", "Ollama model name, for example qwen3:4b")
	promptPath := flag.String(
		"prompt", "", "path to the translation prompt JSON file")
	flag.Usage = func() {
		fmt.Fprintf(
			flag.CommandLine.Output(),
			"Usage: %s -url <url> -model <model> -prompt <prompt.json> <game directory>\n\n",
			filepath.Base(os.Args[0]),
		)

		fmt.Fprintln(flag.CommandLine.Output(), "Options:")
		flag.PrintDefaults()

		fmt.Fprintln(flag.CommandLine.Output())
		fmt.Fprintln(flag.CommandLine.Output(), "Example:")

		fmt.Fprintf(
			flag.CommandLine.Output(),
			"  %s -url http://localhost:11434 -model qwen3:4b "+
				"-prompt ./prompts/en-ru.json /path/to/game\n",
			filepath.Base(os.Args[0]),
		)
	}

	flag.Parse()
	if strings.TrimSpace(*url) == "" {
		fmt.Fprintln(os.Stderr, "Error: -url is required.")
		flag.Usage()
		os.Exit(2)
	}

	if strings.TrimSpace(*model) == "" {
		fmt.Fprintln(os.Stderr, "Error: -model is required.")
		flag.Usage()
		os.Exit(2)
	}

	if strings.TrimSpace(*promptPath) == "" {
		fmt.Fprintln(os.Stderr, "Error: -prompt is required.")
		flag.Usage()
		os.Exit(2)
	}

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Error: exactly one game directory is required.")
		flag.Usage()
		os.Exit(2)
	}

	gameDir, err := filepath.Abs(flag.Arg(0))
	if err != nil {
		log.Fatalf(
			"Failed to resolve the game directory: %v",
			err,
		)
	}

	absolutePromptPath, err := filepath.Abs(*promptPath)
	if err != nil {
		log.Fatalf(
			"Failed to resolve the prompt path: %v",
			err,
		)
	}

	promptConfig, err := ollama.LoadPromptConfig(absolutePromptPath)
	if err != nil {
		log.Fatalf(
			"Failed to load the translation prompt: %v",
			err,
		)
	}

	ollamaClient, err := ollama.NewClient(
		*url,
		*model,
		promptConfig,
	)
	if err != nil {
		log.Fatalf("Failed to create the Ollama client: %v", err)
	}

	cachePath := filepath.Join(gameDir, ".rmloc", "translation-cache.jsonl")
	if err := ollamaClient.EnablePersistentCache(
		cachePath,
	); err != nil {
		log.Fatalf(
			"Failed to load the translation cache: %v",
			err,
		)
	}

	fmt.Printf("Game directory: %s\n", gameDir)
	fmt.Printf("Ollama URL: %s\n", *url)
	fmt.Printf("Ollama model: %s\n", *model)
	fmt.Printf("Prompt file: %s\n", absolutePromptPath)

	if promptConfig.Name != "" {
		fmt.Printf(
			"Prompt profile: %s\n",
			promptConfig.Name,
		)
	}

	fmt.Printf("Translation cache: %s\n\n", cachePath)
	rmmzEngine := rmmz.NewRmmzEngine(ollamaClient)
	application := app.NewApp(*rmmzEngine, 0)
	runErr := application.Run(gameDir)
	ollamaClient.PrintMetrics(os.Stdout)

	if runErr != nil {
		log.Printf("Translation finished with an error: %v", runErr)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("Translation completed successfully.")
}
