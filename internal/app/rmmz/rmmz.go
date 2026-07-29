package rmmz

import (
	"rmloc/internal/ollama"
	"rmloc/internal/progress"
)

type RmmzEngine struct {
	ollamaClient *ollama.Client
	reporter     progress.Reporter
}

func NewRmmzEngine(
	ollamaClient *ollama.Client,
	reporters ...progress.Reporter,
) *RmmzEngine {
	reporter := progress.NewNoopReporter()

	if len(reporters) > 0 &&
		reporters[0] != nil {
		reporter = reporters[0]
	}

	return &RmmzEngine{
		ollamaClient: ollamaClient,
		reporter:     reporter,
	}
}

func (a *RmmzEngine) SetReporter(
	reporter progress.Reporter,
) {
	if reporter == nil {
		a.reporter = progress.NewNoopReporter()
		return
	}

	a.reporter = reporter
}

func (a *RmmzEngine) Reporter() progress.Reporter {
	if a.reporter == nil {
		a.reporter = progress.NewNoopReporter()
	}

	return a.reporter
}
