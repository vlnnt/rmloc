package progress

type NoopReporter struct{}

func NewNoopReporter() Reporter {
	return NoopReporter{}
}

func (NoopReporter) FileDiscovered(
	path string,
	total int,
) {
}

func (NoopReporter) FileStarted(
	path string,
) {
}

func (NoopReporter) FileAdvanced(
	path string,
	count int,
) {
}

func (NoopReporter) FileCompleted(
	path string,
) {
}

func (NoopReporter) FileFailed(
	path string,
	err error,
) {
}

func (NoopReporter) FileCanceled(
	path string,
) {
}
