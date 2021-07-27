package queue

// AbortedJob is a Job that triggers when a Job is timeout or failed.
// If the Job still has retry attempts remaining, this Job won't be triggered.
type AbortedJob struct {
	Err error
	Msg *PersistedJob
}
