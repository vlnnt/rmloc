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
	FileDiscovered(string, int)
	FileStarted(string)
	FileAdvanced(string, int)
	FileCompleted(string)
	FileFailed(string, error)
	FileCanceled(string)
}
