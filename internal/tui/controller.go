package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"rmloc/internal/app"
)

var (
	ErrTranslationAlreadyRunning = errors.New("translation is already running")
	ErrApplicationNotConfigured  = errors.New("translation application is not configured")
)

type RunResult struct {
	Err      error
	Canceled bool
}

type Controller struct {
	mu          sync.Mutex
	application *app.App
	state       *State
	cancel      context.CancelFunc
	done        chan RunResult
}

func NewController(
	application *app.App,
	state *State,
) *Controller {
	if state == nil {
		state = NewState()
	}

	return &Controller{
		application: application,
		state:       state,
	}
}

func (c *Controller) Start(
	gameDir string,
) (<-chan RunResult, error) {
	gameDir = strings.TrimSpace(gameDir)
	if gameDir == "" {
		return nil, fmt.Errorf("game directory is empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.application == nil {
		return nil, ErrApplicationNotConfigured
	}

	if c.cancel != nil {
		return nil, ErrTranslationAlreadyRunning
	}

	c.state.Reset()
	c.state.SetRunning(true)
	c.state.ClearError()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan RunResult, 1)

	c.cancel = cancel
	c.done = done

	go c.run(ctx, gameDir, done)
	return done, nil
}

func (c *Controller) Stop() bool {
	c.mu.Lock()

	cancel := c.cancel
	c.mu.Unlock()

	if cancel == nil {
		return false
	}

	c.state.CancelWaitingFiles()
	cancel()

	return true
}

func (c *Controller) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.cancel != nil
}

func (c *Controller) State() *State {
	return c.state
}

func (c *Controller) Snapshot() StateSnapshot {
	return c.state.Snapshot()
}

func (c *Controller) run(
	ctx context.Context,
	gameDir string,
	done chan RunResult,
) {
	runErr := c.application.RunContext(ctx, gameDir)
	canceled := errors.Is(runErr, context.Canceled) ||
		errors.Is(runErr, context.DeadlineExceeded) ||
		errors.Is(ctx.Err(), context.Canceled) ||
		errors.Is(ctx.Err(), context.DeadlineExceeded)

	c.state.CancelWaitingFiles()
	c.state.SetRunning(false)

	if canceled {
		c.state.ClearError()
	} else if runErr != nil {
		c.state.SetError(runErr)
	} else {
		c.state.ClearError()
	}

	c.mu.Lock()
	if c.done == done {
		c.cancel = nil
		c.done = nil
	}

	c.mu.Unlock()
	done <- RunResult{
		Err:      runErr,
		Canceled: canceled,
	}

	close(done)
}
