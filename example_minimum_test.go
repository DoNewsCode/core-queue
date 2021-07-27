package queue_test

import (
	"context"
	"fmt"
	queue "github.com/DoNewsCode/core-queue"
	"time"
)

func Example_minimum() {
	queueDispatcher := queue.NewQueue(queue.NewInProcessDriver())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var ch = make(chan struct{})
	go queueDispatcher.Consume(ctx)

	queueDispatcher.Subscribe(queue.Listen(queue.From(1), func(ctx context.Context, Job queue.Job) error {
		fmt.Println(Job.Data())
		ch <- struct{}{}
		return nil
	}))
	queueDispatcher.Dispatch(ctx, queue.Adjust(queue.Of(1), queue.Defer(time.Second)))
	queueDispatcher.Dispatch(ctx, queue.Adjust(queue.Of(2), queue.Defer(time.Hour)))

	<-ch

	// Output:
	// 1
}
