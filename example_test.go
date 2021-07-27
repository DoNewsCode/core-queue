package queue_test

import (
	"context"
	"fmt"
	queue "github.com/DoNewsCode/core-queue"
)

func Example() {
	dispatcher := &queue.SyncDispatcher{}
	// Subscribe to int Job.
	dispatcher.Subscribe(queue.Listen(queue.From(0), func(ctx context.Context, Job queue.Job) error {
		fmt.Println(Job.Data())
		return nil
	}))
	// Subscribe to string Job.
	dispatcher.Subscribe(queue.Listen(queue.From(""), func(ctx context.Context, Job queue.Job) error {
		fmt.Println(Job.Data())
		return nil
	}))
	dispatcher.Dispatch(context.Background(), queue.Of(100))
	dispatcher.Dispatch(context.Background(), queue.Of("Job"))
	// Output:
	// 100
	// Job

}
