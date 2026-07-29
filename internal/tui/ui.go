package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"rmloc/internal/app"
	"rmloc/internal/app/rmmz"
	"rmloc/internal/ollama"
	"rmloc/internal/progress"
)

const (
	settingsPageName = "settings"
	progressPageName = "progress"
	exitPageName     = "exit"

	defaultOllamaURL = "http://localhost:11434"
	defaultModel     = "qwen3:4b"
	defaultWorkers   = 2

	uiRefreshInterval = 150 * time.Millisecond
	progressBarWidth  = 32
)

type UI struct {
	application *tview.Application
	pages       *tview.Pages

	settingsRoot   *tview.Flex
	settingsForm   *tview.Form
	settingsStatus *tview.TextView

	gameDirInput *tview.InputField
	ollamaInput  *tview.InputField
	modelInput   *tview.InputField
	promptInput  *tview.InputField
	workersInput *tview.InputField

	progressRoot *tview.Flex
	header       *tview.TextView
	summary      *tview.TextView
	fileTable    *tview.Table
	status       *tview.TextView
	buttons      *tview.Form

	restartButton  *tview.Button
	stopButton     *tview.Button
	settingsButton *tview.Button
	exitButton     *tview.Button

	state      *State
	controller *Controller
	config     Config

	currentPage string

	message      string
	messageColor string

	exitDialogOpen bool
	exitAfterStop  bool

	closed      chan struct{}
	closeOnce   sync.Once
	refreshOnce sync.Once
}

func Run() error {
	ui := NewUI()
	return ui.Run()
}

func NewUI() *UI {
	ui := &UI{
		application: tview.NewApplication(),
		state:       NewState(),
		currentPage: settingsPageName,

		message:      "Готово к запуску перевода.",
		messageColor: "green",

		closed: make(chan struct{}),
	}

	ui.build()
	return ui
}

func (u *UI) Run() error {
	u.application.EnableMouse(true)
	u.application.SetRoot(
		u.pages,
		true,
	)

	u.application.SetFocus(
		u.gameDirInput,
	)

	u.application.SetInputCapture(
		u.captureInput,
	)

	u.startRefreshLoop()
	runErr := runTviewApplication(
		u.application,
	)

	if u.controller != nil &&
		u.controller.IsRunning() {
		u.controller.Stop()
	}

	u.close()

	return runErr
}

func (u *UI) build() {
	u.pages = tview.NewPages()

	u.buildSettingsPage()
	u.buildProgressPage()

	u.pages.AddPage(
		settingsPageName,
		u.settingsRoot,
		true,
		true,
	)

	u.pages.AddPage(
		progressPageName,
		u.progressRoot,
		true,
		false,
	)
}

func (u *UI) buildSettingsPage() {
	currentDirectory, err := os.Getwd()
	if err != nil {
		currentDirectory = "."
	}

	defaultPromptPath := filepath.Join(
		currentDirectory,
		"prompts",
		"en-ru.json",
	)

	title := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	title.SetText(
		"[yellow::b]RMLOC — перевод RPG Maker MZ[-:-:-]\n" +
			"Укажите параметры перевода и нажмите «Старт».",
	)

	title.SetBorder(true)
	title.SetTitle(" RMLOC ")

	u.gameDirInput = tview.NewInputField().
		SetLabel("Папка игры: ").
		SetText(currentDirectory).
		SetFieldWidth(0)

	u.ollamaInput = tview.NewInputField().
		SetLabel("Ollama URL: ").
		SetText(defaultOllamaURL).
		SetFieldWidth(0)

	u.modelInput = tview.NewInputField().
		SetLabel("Модель:     ").
		SetText(defaultModel).
		SetFieldWidth(0)

	u.promptInput = tview.NewInputField().
		SetLabel("Промпт:     ").
		SetText(defaultPromptPath).
		SetFieldWidth(0)

	u.workersInput = tview.NewInputField().
		SetLabel("Потоки:     ").
		SetText(strconv.Itoa(defaultWorkers)).
		SetFieldWidth(8)

	u.settingsForm = tview.NewForm()
	u.settingsForm.AddFormItem(u.gameDirInput)
	u.settingsForm.AddFormItem(u.ollamaInput)
	u.settingsForm.AddFormItem(u.modelInput)
	u.settingsForm.AddFormItem(u.promptInput)
	u.settingsForm.AddFormItem(u.workersInput)

	u.settingsForm.AddButton(
		"Старт [F5]",
		u.startFromSettings,
	)

	u.settingsForm.AddButton(
		"Выход [Q]",
		u.requestExit,
	)

	u.settingsForm.SetButtonsAlign(
		tview.AlignCenter,
	)

	u.settingsForm.SetBorder(true)
	u.settingsForm.SetTitle(" Настройки ")

	u.settingsForm.SetCancelFunc(
		u.requestExit,
	)

	u.settingsStatus = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	u.settingsStatus.SetBorder(true)
	u.settingsStatus.SetTitle(" Состояние ")

	u.setSettingsMessage(
		"Параметры будут проверены после нажатия «Старт».",
		"green",
	)

	help := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	help.SetText(
		"[gray]Tab / Shift+Tab — переход между полями   " +
			"F5 — старт   Q — выход[-]",
	)

	u.settingsRoot = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(title, 4, 0, false).
		AddItem(u.settingsForm, 0, 1, true).
		AddItem(u.settingsStatus, 3, 0, false).
		AddItem(help, 2, 0, false)
}

func (u *UI) buildProgressPage() {
	u.header = tview.NewTextView()
	u.header.SetBorder(true)
	u.header.SetTitle(" Конфигурация ")
	u.header.SetWrap(false)

	u.summary = tview.NewTextView()
	u.summary.SetBorder(true)
	u.summary.SetTitle(" Общий прогресс ")
	u.summary.SetDynamicColors(true)
	u.summary.SetTextAlign(tview.AlignCenter)

	u.fileTable = tview.NewTable()
	u.fileTable.SetBorder(true)
	u.fileTable.SetTitle(" Файлы перевода ")
	u.fileTable.SetFixed(1, 0)
	u.fileTable.SetSelectable(true, false)
	u.fileTable.SetEvaluateAllRows(true)

	u.fileTable.SetSelectedStyle(
		tcell.StyleDefault.
			Foreground(tcell.ColorBlack).
			Background(tcell.ColorLightCyan),
	)

	u.status = tview.NewTextView()
	u.status.SetBorder(true)
	u.status.SetTitle(" Состояние ")
	u.status.SetDynamicColors(true)
	u.status.SetTextAlign(tview.AlignCenter)

	u.buttons = tview.NewForm()
	u.buttons.SetBorder(true)
	u.buttons.SetTitle(" Управление ")
	u.buttons.SetButtonsAlign(tview.AlignCenter)

	u.buttons.AddButton(
		"Повторить [F5]",
		u.restartTranslation,
	)

	u.buttons.AddButton(
		"Стоп [F8]",
		u.stopTranslation,
	)

	u.buttons.AddButton(
		"Настройки [Esc]",
		u.openSettings,
	)

	u.buttons.AddButton(
		"Выход [Q]",
		u.requestExit,
	)

	u.restartButton = u.buttons.GetButton(0)
	u.stopButton = u.buttons.GetButton(1)
	u.settingsButton = u.buttons.GetButton(2)
	u.exitButton = u.buttons.GetButton(3)

	u.progressRoot = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(u.header, 8, 0, false).
		AddItem(u.summary, 4, 0, false).
		AddItem(u.fileTable, 0, 1, true).
		AddItem(u.status, 3, 0, false).
		AddItem(u.buttons, 3, 0, false)

	u.renderTableHeader()
	u.renderProgress()
}

func (u *UI) startFromSettings() {
	config, err := u.readSettings()
	if err != nil {
		u.setSettingsMessage(
			err.Error(),
			"red",
		)

		return
	}

	u.startWithConfig(config)
}

func (u *UI) restartTranslation() {
	if u.controller != nil &&
		u.controller.IsRunning() {
		u.setMessage(
			"Перевод уже выполняется.",
			"yellow",
		)

		u.renderProgress()
		return
	}

	u.startWithConfig(
		u.config,
	)
}

func (u *UI) startWithConfig(
	config Config,
) {
	if u.controller != nil &&
		u.controller.IsRunning() {
		u.setSettingsMessage(
			"Перевод уже выполняется.",
			"yellow",
		)

		return
	}

	u.setSettingsMessage(
		"Проверяем параметры и загружаем промпт...",
		"yellow",
	)

	controller, preparedConfig, err := createController(config)
	if err != nil {
		u.setSettingsMessage(
			fmt.Sprintf(
				"Ошибка: %v",
				err,
			),
			"red",
		)

		return
	}

	u.controller = controller
	u.state = controller.State()
	u.config = preparedConfig

	u.setMessage(
		"Перевод запускается...",
		"yellow",
	)

	u.openProgress()
	done, err := u.controller.Start(
		u.config.GameDir,
	)
	if err != nil {
		u.setMessage(
			fmt.Sprintf(
				"Не удалось запустить перевод: %v",
				err,
			),
			"red",
		)

		u.renderProgress()
		return
	}

	u.setMessage(
		"Перевод запущен.",
		"yellow",
	)

	u.renderProgress()
	go u.waitForResult(
		done,
	)
}

func createController(
	config Config,
) (
	*Controller,
	Config,
	error,
) {
	config = config.Normalized()
	if err := config.Validate(); err != nil {
		return nil, Config{}, err
	}

	promptConfig, err := ollama.LoadPromptConfig(
		config.PromptPath,
	)
	if err != nil {
		return nil, Config{}, fmt.Errorf(
			"загрузить промпт: %w",
			err,
		)
	}

	config.PromptName = strings.TrimSpace(
		promptConfig.Name,
	)

	ollamaClient, err := ollama.NewClient(
		config.OllamaURL,
		config.Model,
		promptConfig,
	)
	if err != nil {
		return nil, Config{}, fmt.Errorf(
			"создать Ollama-клиент: %w",
			err,
		)
	}

	if err := ollamaClient.EnablePersistentCache(
		config.CachePath,
	); err != nil {
		return nil, Config{}, fmt.Errorf(
			"открыть кеш переводов: %w",
			err,
		)
	}

	state := NewState()
	engine := rmmz.NewRmmzEngine(
		ollamaClient,
		state,
	)

	translationApp := app.NewApp(
		*engine,
		config.WorkerCount,
	)

	controller := NewController(
		translationApp,
		state,
	)

	return controller, config, nil
}

func (u *UI) readSettings() (
	Config,
	error,
) {
	gameDir := strings.TrimSpace(
		u.gameDirInput.GetText(),
	)

	if gameDir == "" {
		return Config{}, fmt.Errorf(
			"не указана папка игры",
		)
	}

	absoluteGameDir, err := filepath.Abs(
		gameDir,
	)
	if err != nil {
		return Config{}, fmt.Errorf(
			"определить путь к игре: %w",
			err,
		)
	}

	gameInfo, err := os.Stat(
		absoluteGameDir,
	)
	if err != nil {
		return Config{}, fmt.Errorf(
			"открыть папку игры: %w",
			err,
		)
	}

	if !gameInfo.IsDir() {
		return Config{}, fmt.Errorf(
			"путь игры не является папкой: %s",
			absoluteGameDir,
		)
	}

	ollamaURL := strings.TrimSpace(
		u.ollamaInput.GetText(),
	)

	if ollamaURL == "" {
		return Config{}, fmt.Errorf(
			"не указан адрес Ollama",
		)
	}

	model := strings.TrimSpace(
		u.modelInput.GetText(),
	)

	if model == "" {
		return Config{}, fmt.Errorf(
			"не указана модель Ollama",
		)
	}

	promptPath := strings.TrimSpace(
		u.promptInput.GetText(),
	)

	if promptPath == "" {
		return Config{}, fmt.Errorf(
			"не указан файл промпта",
		)
	}

	absolutePromptPath, err := filepath.Abs(
		promptPath,
	)
	if err != nil {
		return Config{}, fmt.Errorf(
			"определить путь к промпту: %w",
			err,
		)
	}

	promptInfo, err := os.Stat(
		absolutePromptPath,
	)
	if err != nil {
		return Config{}, fmt.Errorf(
			"открыть файл промпта: %w",
			err,
		)
	}

	if promptInfo.IsDir() {
		return Config{}, fmt.Errorf(
			"вместо файла промпта указана папка: %s",
			absolutePromptPath,
		)
	}

	workersText := strings.TrimSpace(
		u.workersInput.GetText(),
	)

	workers, err := strconv.Atoi(
		workersText,
	)
	if err != nil {
		return Config{}, fmt.Errorf(
			"количество потоков должно быть числом",
		)
	}

	if workers <= 0 {
		return Config{}, fmt.Errorf(
			"количество потоков должно быть больше нуля",
		)
	}

	cachePath := filepath.Join(
		absoluteGameDir,
		".rmloc",
		"translation-cache.jsonl",
	)

	config := Config{
		GameDir:     absoluteGameDir,
		OllamaURL:   ollamaURL,
		Model:       model,
		PromptPath:  absolutePromptPath,
		CachePath:   cachePath,
		WorkerCount: workers,
	}

	if err := config.Validate(); err != nil {
		return Config{}, err
	}

	return config, nil
}

func (u *UI) openProgress() {
	u.currentPage = progressPageName

	u.pages.SwitchToPage(
		progressPageName,
	)

	u.application.SetFocus(
		u.fileTable,
	)

	u.renderProgress()
}

func (u *UI) openSettings() {
	if u.controller != nil &&
		u.controller.IsRunning() {
		u.setMessage(
			"Сначала остановите перевод.",
			"yellow",
		)

		u.renderProgress()
		return
	}

	u.currentPage = settingsPageName
	u.pages.SwitchToPage(
		settingsPageName,
	)

	u.application.SetFocus(
		u.gameDirInput,
	)
}

func (u *UI) stopTranslation() {
	if u.controller == nil ||
		!u.controller.Stop() {
		u.setMessage(
			"Сейчас перевод не выполняется.",
			"gray",
		)

		u.renderProgress()
		return
	}

	u.setMessage(
		"Останавливаем перевод...",
		"orange",
	)

	u.renderProgress()
}

func (u *UI) waitForResult(
	done <-chan RunResult,
) {
	if done == nil {
		return
	}

	result, ok := <-done
	if !ok {
		return
	}

	select {
	case <-u.closed:
		return

	default:
	}

	u.application.QueueUpdateDraw(
		func() {
			switch {
			case result.Canceled:
				u.setMessage(
					"Перевод остановлен.",
					"orange",
				)

			case result.Err != nil:
				u.setMessage(
					fmt.Sprintf(
						"Перевод завершился с ошибкой: %v",
						result.Err,
					),
					"red",
				)

			default:
				u.setMessage(
					"Перевод успешно завершён.",
					"green",
				)
			}

			u.renderProgress()

			if u.exitAfterStop {
				u.finishExit()
			}
		},
	)
}

func (u *UI) renderProgress() {
	if u.state == nil {
		return
	}

	snapshot := u.state.Snapshot()

	u.renderHeader()
	u.renderSummary(snapshot)
	u.renderFiles(snapshot)
	u.renderStatus(snapshot)
	u.updateButtons(snapshot.Running)
}

func (u *UI) renderHeader() {
	promptName := strings.TrimSpace(
		u.config.PromptName,
	)

	if promptName == "" {
		promptName = filepath.Base(
			u.config.PromptPath,
		)
	}

	u.header.SetText(
		fmt.Sprintf(
			"Игра:     %s\n"+
				"Ollama:   %s\n"+
				"Модель:   %s\n"+
				"Промпт:   %s\n"+
				"Потоки:   %d\n"+
				"Кеш:      %s",
			u.config.GameDir,
			u.config.OllamaURL,
			u.config.Model,
			promptName,
			u.config.WorkerCount,
			u.config.CachePath,
		),
	)
}

func (u *UI) renderSummary(
	snapshot StateSnapshot,
) {
	percent := calculatePercent(
		snapshot.Completed,
		snapshot.Total,
	)

	completedFiles := countFilesWithStatus(
		snapshot.Files,
		progress.FileStatusCompleted,
	)

	failedFiles := countFilesWithStatus(
		snapshot.Files,
		progress.FileStatusFailed,
	)

	canceledFiles := countFilesWithStatus(
		snapshot.Files,
		progress.FileStatusCanceled,
	)

	color := "yellow"

	switch {
	case failedFiles > 0 || snapshot.Err != nil:
		color = "red"

	case len(snapshot.Files) > 0 &&
		completedFiles == len(snapshot.Files):
		color = "green"

	case canceledFiles > 0 &&
		!snapshot.Running:
		color = "orange"
	}

	u.summary.SetText(
		fmt.Sprintf(
			"[%s]%s[-]  %d%%\n"+
				"Строки: %d/%d   Файлы: %d/%d   "+
				"Ошибки: %d   Отменено: %d   Время: %s",
			color,
			buildProgressBar(percent),
			percent,
			snapshot.Completed,
			snapshot.Total,
			completedFiles,
			len(snapshot.Files),
			failedFiles,
			canceledFiles,
			formatElapsed(snapshot.Elapsed),
		),
	)
}

func (u *UI) renderFiles(
	snapshot StateSnapshot,
) {
	selectedRow, _ := u.fileTable.GetSelection()
	for u.fileTable.GetRowCount() > 0 {
		u.fileTable.RemoveRow(
			u.fileTable.GetRowCount() - 1,
		)
	}

	u.renderTableHeader()

	for index, file := range snapshot.Files {
		row := index + 1
		statusText, statusColor := fileStatusPresentation(
			file.Status,
		)

		errorText := ""
		if file.Err != nil {
			errorText = file.Err.Error()
		}

		u.fileTable.SetCell(
			row,
			0,
			tview.NewTableCell(
				u.displayPath(file.Path),
			).
				SetTextColor(tcell.ColorWhite).
				SetExpansion(1).
				SetMaxWidth(60),
		)

		u.fileTable.SetCell(
			row,
			1,
			tview.NewTableCell(statusText).
				SetTextColor(statusColor).
				SetAlign(tview.AlignCenter).
				SetMaxWidth(14),
		)

		u.fileTable.SetCell(
			row,
			2,
			tview.NewTableCell(
				fileProgressText(file),
			).
				SetTextColor(tcell.ColorLightCyan).
				SetAlign(tview.AlignRight).
				SetMaxWidth(20),
		)

		u.fileTable.SetCell(
			row,
			3,
			tview.NewTableCell(errorText).
				SetTextColor(tcell.ColorRed).
				SetExpansion(1).
				SetMaxWidth(60),
		)
	}

	if len(snapshot.Files) == 0 {
		u.fileTable.SetCell(
			1,
			0,
			tview.NewTableCell(
				"Файлы обнаруживаются после запуска перевода...",
			).
				SetTextColor(tcell.ColorGray).
				SetExpansion(1),
		)

		return
	}

	if selectedRow < 1 {
		selectedRow = 1
	}

	if selectedRow > len(snapshot.Files) {
		selectedRow = len(snapshot.Files)
	}

	u.fileTable.Select(
		selectedRow,
		0,
	)
}

func (u *UI) renderTableHeader() {
	headers := []string{
		"Файл",
		"Статус",
		"Прогресс",
		"Ошибка",
	}

	for column, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAttributes(tcell.AttrBold).
			SetSelectable(false)

		switch column {
		case 0, 3:
			cell.SetExpansion(1)

		case 1:
			cell.SetAlign(tview.AlignCenter)

		case 2:
			cell.SetAlign(tview.AlignRight)
		}

		u.fileTable.SetCell(
			0,
			column,
			cell,
		)
	}
}

func (u *UI) renderStatus(
	snapshot StateSnapshot,
) {
	message := u.message
	color := u.messageColor

	switch {
	case snapshot.Err != nil:
		message = fmt.Sprintf(
			"Ошибка: %v",
			snapshot.Err,
		)

		color = "red"

	case snapshot.Running:
		if message == "" {
			message = "Перевод выполняется..."
		}

		color = "yellow"

	case allFilesCompleted(snapshot.Files):
		message = "Перевод успешно завершён."
		color = "green"
	}

	if message == "" {
		message = "Готово."
		color = "green"
	}

	u.status.SetText(
		fmt.Sprintf(
			"[%s]%s[-]",
			color,
			tview.Escape(message),
		),
	)
}

func (u *UI) updateButtons(
	running bool,
) {
	u.restartButton.SetDisabled(running)
	u.stopButton.SetDisabled(!running)
	u.settingsButton.SetDisabled(running)
	u.exitButton.SetDisabled(false)
}

func (u *UI) startRefreshLoop() {
	u.refreshOnce.Do(
		func() {
			ticker := time.NewTicker(
				uiRefreshInterval,
			)

			go func() {
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						select {
						case <-u.closed:
							return

						default:
						}

						u.application.QueueUpdateDraw(
							func() {
								if u.currentPage ==
									progressPageName {
									u.renderProgress()
								}
							},
						)

					case <-u.closed:
						return
					}
				}
			}()
		},
	)
}

func (u *UI) captureInput(
	event *tcell.EventKey,
) *tcell.EventKey {
	if u.exitDialogOpen {
		if event.Key() == tcell.KeyEscape {
			u.closeExitDialog()

			return nil
		}

		return event
	}

	switch event.Key() {
	case tcell.KeyF5:
		if u.currentPage == settingsPageName {
			u.startFromSettings()
		} else {
			u.restartTranslation()
		}

		return nil

	case tcell.KeyF8:
		u.stopTranslation()

		return nil

	case tcell.KeyEscape:
		if u.currentPage == progressPageName {
			u.openSettings()

			return nil
		}

	case tcell.KeyCtrlC:
		u.requestExit()

		return nil
	}

	switch event.Rune() {
	case 'q', 'Q', 'й', 'Й':
		u.requestExit()

		return nil
	}

	return event
}

func (u *UI) requestExit() {
	if u.exitDialogOpen {
		return
	}
	u.exitDialogOpen = true
	message := "Выйти из программы?"

	if u.controller != nil &&
		u.controller.IsRunning() {
		message = "Перевод ещё выполняется.\n\n" +
			"Остановить перевод и выйти?"
	}

	modal := tview.NewModal().
		SetText(message).
		AddButtons(
			[]string{
				"Отмена",
				"Выйти",
			},
		)

	modal.SetDoneFunc(
		func(
			buttonIndex int,
			buttonLabel string,
		) {
			if buttonLabel == "Выйти" {
				u.beginExit()

				return
			}

			u.closeExitDialog()
		},
	)

	u.pages.AddPage(
		exitPageName,
		modal,
		true,
		true,
	)

	u.application.SetFocus(
		modal,
	)
}

func (u *UI) beginExit() {
	u.closeExitDialog()

	if u.controller != nil &&
		u.controller.IsRunning() {
		u.exitAfterStop = true
		u.controller.Stop()

		u.setMessage(
			"Останавливаем перевод перед выходом...",
			"orange",
		)

		u.openProgress()
		return
	}

	u.finishExit()
}

func (u *UI) closeExitDialog() {
	if !u.exitDialogOpen {
		return
	}

	u.exitDialogOpen = false
	u.pages.RemovePage(
		exitPageName,
	)

	switch u.currentPage {
	case progressPageName:
		u.application.SetFocus(
			u.fileTable,
		)

	default:
		u.application.SetFocus(
			u.gameDirInput,
		)
	}
}

func (u *UI) finishExit() {
	u.close()
	u.application.Stop()
}

func (u *UI) close() {
	u.closeOnce.Do(
		func() {
			close(u.closed)
		},
	)
}

func (u *UI) setSettingsMessage(
	message string,
	color string,
) {
	u.settingsStatus.SetText(
		fmt.Sprintf(
			"[%s]%s[-]",
			color,
			tview.Escape(
				strings.TrimSpace(message),
			),
		),
	)
}

func (u *UI) setMessage(
	message string,
	color string,
) {
	u.message = strings.TrimSpace(message)
	u.messageColor = strings.TrimSpace(color)
}

func (u *UI) displayPath(
	path string,
) string {
	relativePath, err := filepath.Rel(
		u.config.GameDir,
		path,
	)
	if err != nil {
		return path
	}

	if relativePath == "." {
		return filepath.Base(path)
	}

	if relativePath == ".." ||
		strings.HasPrefix(
			relativePath,
			".."+string(filepath.Separator),
		) {
		return path
	}

	return relativePath
}

func calculatePercent(
	completed int,
	total int,
) int {
	if total <= 0 {
		return 0
	}

	if completed < 0 {
		completed = 0
	}

	if completed > total {
		completed = total
	}

	return completed * 100 / total
}

func buildProgressBar(
	percent int,
) string {
	if percent < 0 {
		percent = 0
	}

	if percent > 100 {
		percent = 100
	}

	filled := percent *
		progressBarWidth /
		100

	return strings.Repeat(
		"█",
		filled,
	) + strings.Repeat(
		"░",
		progressBarWidth-filled,
	)
}

func fileProgressText(
	file progress.FileInfo,
) string {
	if file.Total <= 0 {
		switch file.Status {
		case progress.FileStatusCompleted:
			return "готово"

		case progress.FileStatusFailed:
			return "ошибка"

		case progress.FileStatusCanceled:
			return "отменено"

		default:
			return "—"
		}
	}

	completed := file.Completed

	if completed < 0 {
		completed = 0
	}

	if completed > file.Total {
		completed = file.Total
	}

	percent := completed *
		100 /
		file.Total

	return fmt.Sprintf(
		"%d/%d (%d%%)",
		completed,
		file.Total,
		percent,
	)
}

func fileStatusPresentation(
	status progress.FileStatus,
) (
	string,
	tcell.Color,
) {
	switch status {
	case progress.FileStatusWaiting:
		return "Ожидает",
			tcell.ColorGray

	case progress.FileStatusRunning:
		return "Перевод",
			tcell.ColorYellow

	case progress.FileStatusCompleted:
		return "Готово",
			tcell.ColorGreen

	case progress.FileStatusFailed:
		return "Ошибка",
			tcell.ColorRed

	case progress.FileStatusCanceled:
		return "Отменено",
			tcell.ColorOrange

	default:
		return string(status),
			tcell.ColorWhite
	}
}

func countFilesWithStatus(
	files []progress.FileInfo,
	status progress.FileStatus,
) int {
	count := 0

	for _, file := range files {
		if file.Status == status {
			count++
		}
	}

	return count
}

func allFilesCompleted(
	files []progress.FileInfo,
) bool {
	if len(files) == 0 {
		return false
	}

	for _, file := range files {
		if file.Status !=
			progress.FileStatusCompleted {
			return false
		}
	}

	return true
}

func formatElapsed(
	elapsed time.Duration,
) string {
	if elapsed < 0 {
		elapsed = 0
	}

	elapsed = elapsed.Truncate(
		time.Second,
	)

	hours := int(
		elapsed / time.Hour,
	)

	minutes := int(
		elapsed % time.Hour /
			time.Minute,
	)

	seconds := int(
		elapsed % time.Minute /
			time.Second,
	)

	return fmt.Sprintf(
		"%02d:%02d:%02d",
		hours,
		minutes,
		seconds,
	)
}
