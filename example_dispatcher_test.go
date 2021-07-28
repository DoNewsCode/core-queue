package queue_test

import (
	"context"
	"fmt"

	queue "github.com/DoNewsCode/core-queue"
)

func Example() {
	dispatcher := &queue.SyncDispatcher{}
	// Subscribe to int Job.
	dispatcher.Subscribe(queue.Listen(queue.JobFrom(0), func(ctx context.Context, Job queue.Job) error {
		fmt.Println(Job.Data())
		return nil
	}))
	// Subscribe to string Job.
	dispatcher.Subscribe(queue.Listen(queue.JobFrom(""), func(ctx context.Context, Job queue.Job) error {
		fmt.Println(Job.Data())
		return nil
	}))
	dispatcher.Dispatch(context.Background(), queue.JobFrom(100))
	dispatcher.Dispatch(context.Background(), queue.JobFrom("Job"))
	// Output:
	// 100
	// Job

}
