package tui

import (
	"fmt"
	"os"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func runTviewApplication(
	application *tview.Application,
) error {
	if application == nil {
		return fmt.Errorf(
			"tview application is nil",
		)
	}

	var (
		redirectOnce  sync.Once
		restoreOutput func()
		redirectErr   error
	)

	application.SetAfterDrawFunc(
		func(
			screen tcell.Screen,
		) {
			redirectOnce.Do(
				func() {
					restoreOutput, redirectErr = redirectStandardOutput()
				},
			)
		},
	)

	runErr := application.Run()
	if restoreOutput != nil {
		restoreOutput()
	}

	if runErr != nil {
		return runErr
	}

	return redirectErr
}

func redirectStandardOutput() (
	func(),
	error,
) {
	nullOutput, err := os.OpenFile(
		os.DevNull,
		os.O_WRONLY,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"open null output: %w",
			err,
		)
	}

	originalOutput := os.Stdout
	os.Stdout = nullOutput

	var restoreOnce sync.Once
	restore := func() {
		restoreOnce.Do(
			func() {
				os.Stdout = originalOutput
				_ = nullOutput.Close()
			},
		)
	}

	return restore, nil
}
