package queue

// RetryingJob is a Job that triggers when a certain Job failed to be processed, and it is up for retry.
// Note: if retry attempts are exhausted, this Job won't be triggered.
type RetryingJob struct {
	Err error
	Msg *PersistedJob
}
