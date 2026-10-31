package tui

import (
	"sync"
	"time"

	"rmloc/internal/progress"
)

type State struct {
	mu         sync.RWMutex
	files      map[string]*progress.FileInfo
	order      []string
	total      int
	completed  int
	running    bool
	err        error
	startedAt  time.Time
	finishedAt time.Time
}

type StateSnapshot struct {
	Files     []progress.FileInfo
	Total     int
	Completed int
	Running   bool
	Err       error
	Elapsed   time.Duration
}

func NewState() *State {
	return &State{
		files: make(map[string]*progress.FileInfo),
	}
}

func (s *State) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.files = make(map[string]*progress.FileInfo)

	s.order = nil
	s.total = 0
	s.completed = 0
	s.running = false
	s.err = nil

	s.startedAt = time.Time{}
	s.finishedAt = time.Time{}
}

func (s *State) SetRunning(
	running bool,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if running && !s.running {
		s.startedAt = time.Now()
		s.finishedAt = time.Time{}
	}

	if !running &&
		s.running &&
		!s.startedAt.IsZero() {
		s.finishedAt = time.Now()
	}

	s.running = running
}

func (s *State) SetError(
	err error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.err = err
}

func (s *State) ClearError() {
	s.SetError(nil)
}

func (s *State) FileDiscovered(
	path string,
	total int,
) {
	if total < 0 {
		total = 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	file := s.getOrCreateFile(path)

	s.total -= file.Total
	s.completed -= file.Completed

	file.Total = total

	if file.Total > 0 &&
		file.Completed > file.Total {
		file.Completed = file.Total
	}

	s.total += file.Total
	s.completed += file.Completed

	s.normalizeTotals()
}

func (s *State) FileStarted(
	path string,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file := s.getOrCreateFile(path)
	if isTerminalStatus(file.Status) {
		return
	}

	file.Status = progress.FileStatusRunning
	file.Err = nil
}

func (s *State) FileAdvanced(
	path string,
	count int,
) {
	if count <= 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	file := s.getOrCreateFile(path)

	if isTerminalStatus(file.Status) {
		return
	}

	previousCompleted := file.Completed
	file.Completed += count

	if file.Total > 0 &&
		file.Completed > file.Total {
		file.Completed = file.Total
	}

	s.completed += file.Completed - previousCompleted

	s.normalizeTotals()
}

func (s *State) FileCompleted(
	path string,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file := s.getOrCreateFile(path)
	if file.Status == progress.FileStatusCompleted {
		return
	}

	if file.Status == progress.FileStatusFailed {
		return
	}

	previousCompleted := file.Completed
	if file.Total > file.Completed {
		file.Completed = file.Total
	}

	s.completed += file.Completed -
		previousCompleted

	file.Status = progress.FileStatusCompleted
	file.Err = nil

	s.normalizeTotals()
}

func (s *State) FileFailed(
	path string,
	err error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file := s.getOrCreateFile(path)

	if file.Status == progress.FileStatusCompleted {
		return
	}

	file.Status = progress.FileStatusFailed
	file.Err = err
}

func (s *State) FileCanceled(
	path string,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file := s.getOrCreateFile(path)

	switch file.Status {
	case progress.FileStatusCompleted,
		progress.FileStatusFailed,
		progress.FileStatusCanceled:
		return
	}

	file.Status = progress.FileStatusCanceled
	file.Err = nil
}

func (s *State) CancelWaitingFiles() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, path := range s.order {
		file, exists := s.files[path]
		if !exists {
			continue
		}

		if file.Status != progress.FileStatusWaiting {
			continue
		}

		file.Status = progress.FileStatusCanceled
		file.Err = nil
	}
}

func (s *State) Snapshot() StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	files := make(
		[]progress.FileInfo,
		0,
		len(s.order),
	)

	for _, path := range s.order {
		file, exists := s.files[path]
		if !exists {
			continue
		}

		files = append(files, *file)
	}

	elapsed := s.elapsedLocked()
	return StateSnapshot{
		Files:     files,
		Total:     s.total,
		Completed: s.completed,
		Running:   s.running,
		Err:       s.err,
		Elapsed:   elapsed,
	}
}

func (s *State) elapsedLocked() time.Duration {
	if s.startedAt.IsZero() {
		return 0
	}

	endTime := s.finishedAt
	if s.running || endTime.IsZero() {
		endTime = time.Now()
	}

	elapsed := endTime.Sub(s.startedAt)
	if elapsed < 0 {
		return 0
	}

	return elapsed
}

func (s *State) getOrCreateFile(
	path string,
) *progress.FileInfo {
	file, exists := s.files[path]
	if exists {
		return file
	}

	file = &progress.FileInfo{
		Path:   path,
		Status: progress.FileStatusWaiting,
	}

	s.files[path] = file
	s.order = append(s.order, path)

	return file
}

func (s *State) normalizeTotals() {
	if s.total < 0 {
		s.total = 0
	}

	if s.completed < 0 {
		s.completed = 0
	}

	if s.total > 0 &&
		s.completed > s.total {
		s.completed = s.total
	}
}

func isTerminalStatus(
	status progress.FileStatus,
) bool {
	switch status {
	case progress.FileStatusCompleted,
		progress.FileStatusFailed,
		progress.FileStatusCanceled:
		return true

	default:
		return false
	}
}
