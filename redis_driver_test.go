package queue_test

import (
	"context"
	queue "github.com/DoNewsCode/core-queue"
	"sync"
	"testing"
)

func setUpInProcessQueueBenchmark(wg *sync.WaitGroup) (*queue.Queue, func()) {
	queueDispatcher := queue.NewQueue(queue.NewInProcessDriver())
	ctx, cancel := context.WithCancel(context.Background())
	go queueDispatcher.Consume(ctx)
	queueDispatcher.Subscribe(queue.Listen(queue.From(1), func(ctx context.Context, Job queue.Job) error {
		wg.Done()
		return nil
	}))
	return queueDispatcher, cancel
}

func setUpRedisQueueBenchmark(wg *sync.WaitGroup) (*queue.Queue, func()) {
	queueDispatcher := queue.NewQueue(&queue.RedisDriver{})
	ctx, cancel := context.WithCancel(context.Background())
	go queueDispatcher.Consume(ctx)
	queueDispatcher.Subscribe(queue.Listen(queue.From(1), func(ctx context.Context, Job queue.Job) error {
		wg.Done()
		return nil
	}))
	return queueDispatcher, cancel
}

func BenchmarkRedisQueue(b *testing.B) {
	var wg sync.WaitGroup
	dispatcher, cancel := setUpRedisQueueBenchmark(&wg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		dispatcher.Dispatch(context.Background(), queue.Adjust(queue.Of(1)))
	}
	wg.Wait()
	cancel()
}

func BenchmarkInProcessQueue(b *testing.B) {
	var wg sync.WaitGroup
	dispatcher, cancel := setUpInProcessQueueBenchmark(&wg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		dispatcher.Dispatch(context.Background(), queue.Adjust(queue.Of(1)))
	}
	wg.Wait()
	cancel()
}
