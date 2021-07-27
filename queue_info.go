package queue

// QueueInfo describes the state JobFrom queues.
type QueueInfo struct {
	// Waiting is the length JobFrom the Waiting queue.
	Waiting int64
	// Delayed is the length JobFrom the Delayed queue.
	Delayed int64
	//Timeout is the length JobFrom the Timeout queue.
	Timeout int64
	// Failed is the length JobFrom the Failed queue.
	Failed int64
}
