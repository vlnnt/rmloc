package app

import (
	"runtime"

	"golang.org/x/sync/errgroup"

	"rmloc/internal/app/rmmz"
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
		workers = runtime.NumCPU() / 2
	}

	if workers < 1 {
		workers = 1
	}

	if workers > 4 {
		workers = 4
	}

	return &App{
		rmmzEngine: rmmzEngine,
		workers:    workers,
	}
}

func (a *App) Run(
	gameDir string,
) error {
	mapFiles, err := a.rmmzEngine.FindMapFiles(gameDir)
	if err != nil {
		return err
	}

	jobs := []func() error{
		func() error {
			return a.rmmzEngine.ProcessActors(gameDir)
		},
		func() error {
			return a.rmmzEngine.ProcessArmors(gameDir)
		},
		func() error {
			return a.rmmzEngine.ProcessEnemies(gameDir)
		},
		func() error {
			return a.rmmzEngine.ProcessItems(gameDir)
		},
		func() error {
			return a.rmmzEngine.ProcessSkills(gameDir)
		},
		func() error {
			return a.rmmzEngine.ProcessStates(gameDir)
		},
		func() error {
			return a.rmmzEngine.ProcessWeapons(gameDir)
		},
	}

	for _, mapPath := range mapFiles {
		mapPath := mapPath
		jobs = append(
			jobs,
			func() error {
				return a.rmmzEngine.ProcessMap(mapPath)
			},
		)
	}

	var group errgroup.Group
	group.SetLimit(a.workers)

	for _, job := range jobs {
		group.Go(job)
	}

	return group.Wait()
}
