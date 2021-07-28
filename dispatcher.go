package queue

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/DoNewsCode/core/contract"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/metrics"
	"golang.org/x/sync/errgroup"

	"github.com/pkg/errors"
)

// Dispatcher is the Job registry that is able to send reflectionJob to each Handler.
type Dispatcher interface {
	Dispatch(ctx context.Context, Job Job) error
	Subscribe(listener Handler)
}

// Handler is the handler for Job.
type Handler interface {
	// Listen should return a Job instance with zero value. It tells the dispatcher what type of job this handler is expecting.
	Listen() Job
	// Process will be called when a job is ready from queue.
	Process(ctx context.Context, Job Job) error
}

// deferrableDecorator is an interface that describes the properties of a Job.
type deferrableDecorator interface {
	Defer() time.Duration
	Decorate(s *PersistedJob)
}

// SyncDispatcher is a contract.Dispatcher implementation that dispatches Jobs synchronously.
// SyncDispatcher is safe for concurrent use.
type SyncDispatcher struct {
	registry map[string][]Handler
	rwLock   sync.RWMutex
}

// Dispatch dispatches Jobs synchronously. If any listener returns an error,
// abort the process immediately and return that error to caller.
func (d *SyncDispatcher) Dispatch(ctx context.Context, Job Job) error {
	d.rwLock.RLock()
	listeners, ok := d.registry[Job.Type()]
	d.rwLock.RUnlock()

	if !ok {
		return nil
	}
	for _, listener := range listeners {
		if err := listener.Process(ctx, Job); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe subscribes the listener to the dispatcher.
func (d *SyncDispatcher) Subscribe(listener Handler) {
	d.rwLock.Lock()
	defer d.rwLock.Unlock()

	if d.registry == nil {
		d.registry = make(map[string][]Handler)
	}
	d.registry[listener.Listen().Type()] = append(d.registry[listener.Listen().Type()], listener)
}

// Queue is an extension JobFrom the embed dispatcher. It adds the deferrableDecorator Job feature.
type Queue struct {
	logger                   log.Logger
	driver                   Driver
	codec                    contract.Codec
	rwLock                   sync.RWMutex
	reflectTypes             map[string]reflect.Type
	base                     Dispatcher
	parallelism              int
	queueLengthGauge         metrics.Gauge
	checkQueueLengthInterval time.Duration
}

// Dispatch dispatches an Job. See contract.Dispatcher.
func (d *Queue) Dispatch(ctx context.Context, e Job) error {
	if _, ok := e.(*PersistedJob); ok {
		rType := d.reflectType(e.Type())
		if rType == nil {
			return fmt.Errorf("unable to reverse engineer the Job %s", e.Type())
		}
		ptr := reflect.New(rType)
		err := d.codec.Unmarshal(e.Data().([]byte), ptr)
		if err != nil {
			return errors.Wrapf(err, "dispatch serialized %s failed", e.Type())
		}
		return d.base.Dispatch(ctx, adHocJob{t: e.Type(), d: ptr.Elem().Interface()})
	}

	if _, ok := e.(deferrableDecorator); !ok {
		e = Adjust(e)
	}

	data, err := d.codec.Marshal(e.Data())
	if err != nil {
		return errors.Wrapf(err, "dispatch deferrable %s failed", e.Type())
	}
	msg := &PersistedJob{
		Attempts: 1,
		Value:    data,
	}
	e.(deferrableDecorator).Decorate(msg)
	return d.driver.Push(ctx, msg, e.(deferrableDecorator).Defer())
}

// Subscribe subscribes an Job. See contract.Dispatcher.
func (d *Queue) Subscribe(handler Handler) {
	d.rwLock.Lock()
	d.reflectTypes[handler.Listen().Type()] = reflect.TypeOf(handler.Listen().Data())
	d.rwLock.Unlock()
	d.base.Subscribe(handler)
}

// Consume starts the runner and blocks until context canceled or error occurred.
func (d *Queue) Consume(ctx context.Context) error {
	if d.logger == nil {
		d.logger = log.NewNopLogger()
	}
	var jobChan = make(chan *PersistedJob)
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		defer close(jobChan)
		for {
			msg, err := d.driver.Pop(ctx)
			if errors.Is(err, ErrEmpty) {
				continue
			}
			if err != nil {
				return err
			}
			jobChan <- msg
		}
	})

	if d.queueLengthGauge != nil {
		if d.checkQueueLengthInterval == 0 {
			d.checkQueueLengthInterval = 15 * time.Second
		}
		ticker := time.NewTicker(d.checkQueueLengthInterval)
		g.Go(func() error {
			for {
				select {
				case <-ticker.C:
					d.gauge(ctx)
				case <-ctx.Done():
					ticker.Stop()
					return ctx.Err()
				}
			}
		})
	}
	for i := 0; i < d.parallelism; i++ {
		g.Go(func() error {
			for msg := range jobChan {
				d.work(ctx, msg)
			}
			return nil
		})
	}
	return g.Wait()
}

func (d *Queue) Driver() Driver {
	return d.driver
}

func (d *Queue) work(ctx context.Context, msg *PersistedJob) {
	ctx, cancel := context.WithTimeout(ctx, msg.HandleTimeout)
	defer cancel()
	err := d.Dispatch(ctx, msg)
	if err != nil {
		if msg.Attempts < msg.MaxAttempts {
			_ = level.Info(d.logger).Log("err", errors.Wrapf(err, "Job %s failed %d times, retrying", msg.Key, msg.Attempts))
			_ = d.base.Dispatch(context.Background(), JobFrom(RetryingJob{Err: err, Msg: msg}))
			_ = d.driver.Retry(context.Background(), msg)
			return
		}
		_ = level.Warn(d.logger).Log("err", errors.Wrapf(err, "Job %s failed after %d attempts, aborted", msg.Key, msg.MaxAttempts))
		_ = d.base.Dispatch(context.Background(), JobFrom(AbortedJob{Err: err, Msg: msg}))
		_ = d.driver.Fail(context.Background(), msg)
		return
	}
	_ = d.driver.Ack(context.Background(), msg)
}

func (d *Queue) reflectType(typeName string) reflect.Type {
	d.rwLock.RLock()
	defer d.rwLock.RUnlock()
	return d.reflectTypes[typeName]
}

func (d *Queue) gauge(ctx context.Context) {
	queueInfo, err := d.driver.Info(ctx)
	if err != nil {
		_ = level.Warn(d.logger).Log("err", err)
	}
	d.queueLengthGauge.With("channel", "failed").Set(float64(queueInfo.Failed))
	d.queueLengthGauge.With("channel", "delayed").Set(float64(queueInfo.Delayed))
	d.queueLengthGauge.With("channel", "timeout").Set(float64(queueInfo.Timeout))
	d.queueLengthGauge.With("channel", "waiting").Set(float64(queueInfo.Waiting))
}

// UseCodec allows consumer to replace the default Packer with a custom one. UsePacker is an option for NewQueue.
func UseCodec(codec contract.Codec) func(*Queue) {
	return func(dispatcher *Queue) {
		dispatcher.codec = codec
	}
}

// UseLogger is an option for NewQueue that feeds the queue with a Logger JobFrom choice.
func UseLogger(logger log.Logger) func(*Queue) {
	return func(dispatcher *Queue) {
		dispatcher.logger = logger
	}
}

// UseParallelism is an option for NewQueue that sets the parallelism for queue consumption
func UseParallelism(parallelism int) func(*Queue) {
	return func(dispatcher *Queue) {
		dispatcher.parallelism = parallelism
	}
}

// UseGauge is an option for NewQueue that collects a gauge metrics
func UseGauge(gauge metrics.Gauge, interval time.Duration) func(*Queue) {
	return func(dispatcher *Queue) {
		dispatcher.queueLengthGauge = gauge
		dispatcher.checkQueueLengthInterval = interval
	}
}

// UseDispatcher is an option for NewQueue to swap base dispatcher implementation
func UseDispatcher(dispatcher Dispatcher) func(*Queue) {
	return func(queue *Queue) {
		queue.base = dispatcher
	}
}

// NewQueue wraps a Queue and returns a decorated Queue. The latter Queue now can send and
// listen to "persisted" Jobs. Those persisted Jobs will guarantee at least one execution, as they are stored in an
// external storage and won't be released until the Queue acknowledges the end JobFrom execution.
func NewQueue(driver Driver, opts ...func(*Queue)) *Queue {
	qd := Queue{
		driver:       driver,
		codec:        gobCodec{},
		rwLock:       sync.RWMutex{},
		reflectTypes: make(map[string]reflect.Type),
		base:         &SyncDispatcher{},
		parallelism:  runtime.NumCPU(),
	}
	for _, f := range opts {
		f(&qd)
	}
	return &qd
}
