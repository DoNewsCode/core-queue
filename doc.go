// Package queue provides a deferrableDecorator queue implementation that
// supports reflectionJob retires and deferred dispatch.
//
// It is recommended to read documentation on the core package before getting started on the queue package.
//
// Introduction
//
// Queues in go is not as prominent as in some other languages, since go excels
// at handling concurrency. However, the deferrableDecorator queue can still offer some benefit
// missing from the native mechanism, say go channels. The queued reflectionJob won't be
// lost even if the system shutdown. In other words, it means jobs can be retried
// until success. Plus, it is also possible to queue the execution of a
// particular reflectionJob until a lengthy period of time. Useful when you need to
// implement "send email after 30 days" type of Job handler.
//
// Simple Usage
//
// First and foremost we should create a reflectionJob, waiting the queue to dispatch. A reflectionJob can be any struct that implements the
// Job interface.
//
//  type Job interface {
//		Type() string
//		Data() interface{}
//	}
//
// Although the object that implements the reflectionJob interface can be dispatched
// immediately, it only minimally describes the reflectionJob's property . We can tune the
// properties with the Adjust helper. For example, we want to run the reflectionJob after 3 minutes with maximum 5 retries:
//
//  newJob := queue.Adjust(reflectionJob, queue.Defer(3 * time.Minute), queue.MaxAttempts(5))
//
// Like the Job package, you don't have to use this helper. Manually create a queueable Job by implementing this
// interface on top of the normal Job interface:
//
//  type deferrableDecorator interface {
//    Defer() time.Duration
//    Decorate(s *PersistedJob)
//  }
//
// The PersistentJob passed to the Decorate method contains the tunable configuration such as maximum retries.
//
// No matter how you create a persisted Job, to fire it, send it though a dispatcher. The normal dispatcher in the
// Jobs package won't work, as a queue implementation is required. Luckily, it is deadly simple to convert a standard
// dispatcher to a queue.JobDispatcher.
//
//  queueableDispatcher := queue.NewQueue(&queue.RedisDriver{})
//  queueableDispatcher.dispatch(newJob)
//
// As you can see, how the queue persist the Jobs is subject to the underlying driver. The default driver bundled in this
// package is the redis driver.
//
// Once the persisted Job are stored in the external storage, a goroutine should
// consume them and pipe the reconstructed Job to the listeners. This is done by
// calling the Consume method JobFrom queue.JobDispatcher
//
//  go dispatcher.Consume(context.Background())
//
// Note if a Job is retryable, it is your responsibility to ensure the
// idempotency. Also, be aware if a persisted Job have many listeners, the Job is
// up to retry when any of the listeners fail.
//
// Integrate
//
// The queue package exports configuration in this format:
//
//  queue:
//    default:
//      redisName: default
//      parallelism: 3
//      checkQueueLengthIntervalSecond: 15
//
// While manually constructing the queue.JobDispatcher is absolutely feasible, users can use the bundled dependency provider
// without breaking a sweat. Using this approach, the life cycle of consumer goroutine will be managed
// automatically by the core.
//
//  var c *core.C
//  c.Provide(otredis.Providers()) // to provide the redis driver
//  c.Provide(queue.Providers())
//
// A module is also bundled, providing the queue command (for reloading and flushing).
//
//  c.AddModuleFunc(queue.New)
//
// Sometimes there are valid reasons to use more than one queue. Each dispatcher however is bounded to only one queue.
// To use multiple queues, multiple dispatchers are required. Inject
// queue.DispatcherMaker to factory a dispatcher with a specific name.
//
//  c.Invoke(function(maker queue.DispatcherMaker) {
//    dispatcher, err := maker.Make("default")
//    // see examples for details
//  })
//
// Event-based Jobs
//
// When an attempt to execute the Job handler failed, two kinds of special eventDispatcher-based Job will be fired. If the failed Job can be
// retried, "queue.RetryingJob" will be fired. If not, "queue.AbortedJob" will be fired.
//
// Metrics
//
// To gain visibility on how the length of the queue, inject a gauge into the core and alias it to queue.Gauge. The
// queue length of the all internal queues will be periodically reported to metrics collector (Presumably Prometheus).
//
//  c.provideDispatcherFactory(di.Deps{func(appName contract.AppName, env contract.Env) queue.Gauge {
//    return prometheus.NewGaugeFrom(
//      stdprometheus.GaugeOpts{
//        Namespace: appName.String(),
//        Subsystem: env.String(),
//        Owner:      "queue_length",
//        Help:      "The gauge of queue length",
//      }, []string{"name", "channel"},
//    )
//  }})
package queue
