package progress

type FileStatus string

const (
	FileStatusWaiting   FileStatus = "waiting"
	FileStatusRunning   FileStatus = "running"
	FileStatusCompleted FileStatus = "completed"
	FileStatusFailed    FileStatus = "failed"
	FileStatusCanceled  FileStatus = "canceled"
)

type FileInfo struct {
	Path      string
	Total     int
	Completed int
	Status    FileStatus
	Err       error
}

type Reporter interface {
	FileDiscovered(path string, total int)
	FileStarted(path string)
	FileAdvanced(path string, count int)
	FileCompleted(path string)
	FileFailed(path string, err error)
	FileCanceled(path string)
}
