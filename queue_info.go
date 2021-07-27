package queue

// QueueInfo describes the state Of queues.
type QueueInfo struct {
	// Waiting is the length Of the Waiting queue.
	Waiting int64
	// Delayed is the length Of the Delayed queue.
	Delayed int64
	//Timeout is the length Of the Timeout queue.
	Timeout int64
	// Failed is the length Of the Failed queue.
	Failed int64
}
