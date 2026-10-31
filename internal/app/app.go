package app

import (
	"context"

	"golang.org/x/sync/errgroup"

	"rmloc/internal/app/rmmz"
)

const (
	defaultWorkers = 2
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

	translationFiles, err := a.rmmzEngine.DiscoverTranslationFilesContext(ctx, gameDir)
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
			return a.rmmzEngine.ProcessActorsContext(ctx, gameDir)
		},
		func(ctx context.Context) error {
			return a.rmmzEngine.ProcessArmorsContext(ctx, gameDir)
		},
		func(ctx context.Context) error {
			return a.rmmzEngine.ProcessEnemiesContext(ctx, gameDir)
		},
		func(ctx context.Context) error {
			return a.rmmzEngine.ProcessItemsContext(ctx, gameDir)
		},
		func(ctx context.Context) error {
			return a.rmmzEngine.ProcessSkillsContext(ctx, gameDir)
		},
		func(ctx context.Context) error {
			return a.rmmzEngine.ProcessStatesContext(ctx, gameDir)
		},
		func(ctx context.Context) error {
			return a.rmmzEngine.ProcessWeaponsContext(ctx, gameDir)
		},
	}

	for _, mapPath := range mapFiles {
		currentMapPath := mapPath
		jobs = append(
			jobs,
			func(ctx context.Context) error {
				return a.rmmzEngine.ProcessMapContext(ctx, currentMapPath)
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
