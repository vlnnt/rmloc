package rmmz

import "rmloc/internal/ollama"

type RmmzEngine struct {
	ollamaClient *ollama.Client
}

func NewRmmzEngine(
	ollamaClient *ollama.Client,
) *RmmzEngine {
	return &RmmzEngine{
		ollamaClient: ollamaClient,
	}
}
