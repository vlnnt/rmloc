package rmmz

import (
	"context"
	"fmt"
)

func (a *RmmzEngine) DiscoverTranslationFiles(
	ctx context.Context,
	gameDir string,
) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	files := make([]string, 0, 16)
	seen := make(map[string]struct{})
	addFile := func(path string) {
		if path == "" {
			return
		}

		if _, exists := seen[path]; exists {
			return
		}

		seen[path] = struct{}{}
		files = append(files, path)
	}

	actorsPath, err := findActorsFile(gameDir)
	if err != nil {
		return nil, fmt.Errorf(
			"discover Actors.json: %w",
			err,
		)
	}

	addFile(actorsPath)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	armorsPath, err := findArmorsFile(gameDir)
	if err != nil {
		return nil, fmt.Errorf(
			"discover Armors.json: %w",
			err,
		)
	}

	addFile(armorsPath)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	classesPath, err := findClassesFile(gameDir)
	if err != nil {
		return nil, fmt.Errorf(
			"discover Classes.json: %w",
			err,
		)
	}

	addFile(classesPath)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	commonEventsPath, err := findCommonEventsFile(gameDir)
	if err != nil {
		return nil, fmt.Errorf(
			"discover CommonEvents.json: %w",
			err,
		)
	}

	addFile(commonEventsPath)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	enemiesPath, err := findEnemiesFile(gameDir)
	if err != nil {
		return nil, fmt.Errorf(
			"discover Enemies.json: %w",
			err,
		)
	}

	addFile(enemiesPath)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	itemsPath, err := findItemsFile(gameDir)
	if err != nil {
		return nil, fmt.Errorf(
			"discover Items.json: %w",
			err,
		)
	}

	addFile(itemsPath)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	skillsPath, err := findSkillsFile(gameDir)
	if err != nil {
		return nil, fmt.Errorf(
			"discover Skills.json: %w",
			err,
		)
	}

	addFile(skillsPath)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	statesPath, err := findStatesFile(gameDir)
	if err != nil {
		return nil, fmt.Errorf(
			"discover States.json: %w",
			err,
		)
	}

	addFile(statesPath)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	systemPath, err := findSystemFile(gameDir)
	if err != nil {
		return nil, fmt.Errorf(
			"discover System.json: %w",
			err,
		)
	}

	addFile(systemPath)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	troopsPath, err := findTroopsFile(gameDir)
	if err != nil {
		return nil, fmt.Errorf(
			"discover Troops.json: %w",
			err,
		)
	}

	addFile(troopsPath)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	weaponsPath, err := findWeaponsFile(gameDir)
	if err != nil {
		return nil, fmt.Errorf(
			"discover Weapons.json: %w",
			err,
		)
	}

	addFile(weaponsPath)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	mapFiles, err := a.FindMapFilesContext(ctx, gameDir)
	if err != nil {
		return nil, fmt.Errorf(
			"discover map files: %w",
			err,
		)
	}

	for _, mapPath := range mapFiles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		addFile(mapPath)
	}

	return files, nil
}
