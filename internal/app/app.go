package app

import (
	"context"

	"golang.org/x/sync/errgroup"

	"rmloc/internal/app/rmmz"
)

const (
	defaultWorkers = 1
	maxWorkers     = 4
)

type App struct {
	rmmzEngine rmmz.RmmzEngine
	workers    int
}

func NewApp(
	rmmzEngine rmmz.RmmzEngine,
	workers int,
) *App {
	if workers <= 0 {
		workers = defaultWorkers
	}

	if workers > maxWorkers {
		workers = maxWorkers
	}

	return &App{
		rmmzEngine: rmmzEngine,
		workers:    workers,
	}
}

func (a *App) Run(
	gameDir string,
) error {
	return a.RunContext(context.Background(), gameDir)
}

func (a *App) RunContext(
	ctx context.Context,
	gameDir string,
) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	translationFiles, err := a.rmmzEngine.DiscoverTranslationFiles(ctx, gameDir)
	if err != nil {
		return err
	}

	reporter := a.rmmzEngine.Reporter()
	for _, filePath := range translationFiles {
		if err := ctx.Err(); err != nil {
			return err
		}

		reporter.FileDiscovered(filePath, 0)
	}

	mapFiles, err := a.rmmzEngine.FindMapFilesContext(ctx, gameDir)
	if err != nil {
		return err
	}

	type job func(context.Context) error
	jobs := []job{
		func(ctx context.Context) error {
			return a.rmmzEngine.ProcessActors(ctx, gameDir)
		},
		func(ctx context.Context) error {
			return a.rmmzEngine.ProcessArmors(ctx, gameDir)
		},
		func(ctx context.Context) error {
			return a.rmmzEngine.ProcessCommonEvents(ctx, gameDir)
		},
		func(ctx context.Context) error {
			return a.rmmzEngine.ProcessEnemies(ctx, gameDir)
		},
		func(ctx context.Context) error {
			return a.rmmzEngine.ProcessItems(ctx, gameDir)
		},
		func(ctx context.Context) error {
			return a.rmmzEngine.ProcessSkills(ctx, gameDir)
		},
		func(ctx context.Context) error {
			return a.rmmzEngine.ProcessStates(ctx, gameDir)
		},
		func(ctx context.Context) error {
			return a.rmmzEngine.ProcessWeapons(ctx, gameDir)
		},
	}

	for _, mapPath := range mapFiles {
		currentMapPath := mapPath
		jobs = append(
			jobs,
			func(ctx context.Context) error {
				return a.rmmzEngine.ProcessMap(ctx, currentMapPath)
			},
		)
	}

	group, groupContext := errgroup.WithContext(ctx)
	group.SetLimit(a.workers)

	for _, currentJob := range jobs {
		if err := groupContext.Err(); err != nil {
			break
		}

		currentJob := currentJob
		group.Go(
			func() error {
				if err := groupContext.Err(); err != nil {
					return err
				}

				return currentJob(groupContext)
			},
		)
	}

	return group.Wait()
}
