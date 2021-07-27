package queue

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/DoNewsCode/core/logging"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

type MockListener func(ctx context.Context, Job Job) error

func (m MockListener) Listen() []Job {
	return From(MockJob{})
}

func (m MockListener) Process(ctx context.Context, Job Job) error {
	return m(ctx, Job)
}

type RetryingListener func(ctx context.Context, Job Job) error

func (m RetryingListener) Listen() []Job {
	return From(RetryingJob{})
}

func (m RetryingListener) Process(ctx context.Context, Job Job) error {
	return m(ctx, Job)
}

type AbortedListener func(ctx context.Context, Job Job) error

func (m AbortedListener) Listen() []Job {
	return From(AbortedJob{})
}

func (m AbortedListener) Process(ctx context.Context, Job Job) error {
	return m(ctx, Job)
}

type MockJob struct {
	Value  string
	Called *bool
}

func TestMain(m *testing.M) {
	_, ok := getDefaultRedisAddrs()

	if !ok {
		fmt.Println("Set env REDIS_ADDR to run queue tests")
		os.Exit(0)
	}

	os.Exit(m.Run())
}

func setUp() *Queue {
	envDefaultRedisAddrs, _ := getDefaultRedisAddrs()
	s := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: envDefaultRedisAddrs,
	})
	driver := RedisDriver{
		Logger:      logging.NewLogger("logfmt"),
		RedisClient: s,
		ChannelConfig: ChannelConfig{
			Delayed:  "delayed",
			Failed:   "failed",
			Reserved: "reserved",
			Waiting:  "waiting",
			Timeout:  "timeout",
		},
		PopTimeout: time.Second,
		Packer:     gobCodec{},
	}
	dispatcher := NewQueue(&driver, UseLogger(logging.NewLogger("logfmt")))
	return dispatcher
}

func tearDown() {
	channel := ChannelConfig{
		Delayed:  "delayed",
		Failed:   "failed",
		Reserved: "reserved",
		Waiting:  "waiting",
		Timeout:  "timeout",
	}
	envDefaultRedisAddrs, _ := getDefaultRedisAddrs()
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: envDefaultRedisAddrs})
	redisClient.Del(context.Background(), channel.Delayed)
	redisClient.Del(context.Background(), channel.Failed)
	redisClient.Del(context.Background(), channel.Reserved)
	redisClient.Del(context.Background(), channel.Waiting)
	redisClient.Del(context.Background(), channel.Timeout)
}

func TestDispatcher_work(t *testing.T) {
	rand.Seed(time.Now().Unix())

	cases := []struct {
		name        string
		value       Job
		ln          MockListener
		maxAttempts int
		check       func(int, int)
	}{
		{
			"simple message",
			Of(MockJob{Value: "hello"}),
			func(ctx context.Context, Job Job) error {
				assert.IsType(t, MockJob{}, Job.Data())
				assert.Equal(t, "hello", Job.Data().(MockJob).Value)
				return nil
			},
			1,
			func(retries, failed int) {
				assert.Equal(t, 0, retries)
				assert.Equal(t, 0, failed)
			},
		},
		{
			"retry message",
			Of(MockJob{Value: "hello"}),
			func(ctx context.Context, Job Job) error {
				assert.IsType(t, MockJob{}, Job.Data())
				assert.Equal(t, "hello", Job.Data().(MockJob).Value)
				return errors.New("foo")
			},
			2,
			func(retries, failed int) {
				assert.Equal(t, 1, retries)
				assert.Equal(t, 0, failed)
			},
		},
		{
			"fail message",
			Of(MockJob{Value: "hello"}),
			func(ctx context.Context, Job Job) error {
				assert.IsType(t, MockJob{}, Job.Data())
				assert.Equal(t, "hello", Job.Data().(MockJob).Value)
				return errors.New("foo")
			},
			1,
			func(retries, failed int) {
				assert.Equal(t, 0, retries)
				assert.Equal(t, 1, failed)
			},
		},
	}
	for _, cc := range cases {
		c := cc
		t.Run(c.name, func(t *testing.T) {
			retries := 0
			failed := 0
			dispatcher := setUp()
			defer tearDown()
			dispatcher.Subscribe(c.ln)
			dispatcher.Subscribe(RetryingListener(func(ctx context.Context, Job Job) error {
				retries++
				return nil
			}))
			dispatcher.Subscribe(AbortedListener(func(ctx context.Context, Job Job) error {
				failed++
				return nil
			}))
			msg, err := dispatcher.codec.Marshal(c.value.Data())
			assert.NoError(t, err)
			dispatcher.work(context.Background(), &PersistedJob{
				Key:         c.value.Type(),
				Value:       msg,
				MaxAttempts: c.maxAttempts,
				Attempts:    1,
			})
			c.check(retries, failed)
		})
	}
}

func TestDispatcher_Consume(t *testing.T) {
	consumer := setUp()
	defer tearDown()

	var firstTry = make(chan struct{}, 1)
	var called = make(chan string)
	cases := []struct {
		name   string
		evt    Job
		ln     MockListener
		called func()
	}{
		{
			"ordinary message",
			Of(MockJob{Value: "hello"}),
			func(ctx context.Context, Job Job) error {
				assert.IsType(t, MockJob{}, Job.Data())
				assert.Equal(t, "hello", Job.Data().(MockJob).Value)
				called <- "ordinary message"
				return nil
			},
			func() {
				str := <-called
				assert.Equal(t, "ordinary message", str)
			},
		},
		{
			"persist message",
			Adjust(Of(MockJob{Value: "hello"})),
			func(ctx context.Context, Job Job) error {
				assert.IsType(t, MockJob{}, Job.Data())
				assert.Equal(t, "hello", Job.Data().(MockJob).Value)
				called <- "persist message"
				return nil
			},
			func() {
				str := <-called
				assert.Equal(t, "persist message", str)
			},
		},
		{
			"deferred message",
			Adjust(Of(MockJob{Value: "hello", Called: new(bool)}), Defer(2*time.Second)),
			func(ctx context.Context, Job Job) error {
				called <- "deferred message"
				return nil
			},
			func() {
				var str string
				select {
				case str = <-called:
				case <-time.After(time.Second):
				}
				assert.NotEqual(t, "deferred message", str)
				str = <-called
				assert.Equal(t, "deferred message", str)
			},
		},
		{
			"deferred message but called",
			Adjust(Of(MockJob{Value: "hello", Called: new(bool)}), Defer(time.Second)),
			func(ctx context.Context, Job Job) error {
				called <- "deferred message but called"
				return nil
			},
			func() {
				var str string
				select {
				case str = <-called:
				case <-time.After(2 * time.Second):
				}
				assert.Equal(t, "deferred message but called", str)
			},
		},
		{
			"failed message",
			Adjust(Of(MockJob{Value: "hello"})),
			func(ctx context.Context, Job Job) error {
				defer func() {
					called <- "failed message"
				}()
				return errors.New("some err")
			},
			func() {
				<-called
				time.Sleep(100 * time.Millisecond)
				info, _ := consumer.driver.Info(context.Background())
				assert.Equal(t, int64(1), info.Failed)
				err := consumer.driver.Flush(context.Background(), "failed")
				assert.NoError(t, err)
			},
		},
		{
			"retry message",
			Adjust(Of(MockJob{Value: "hello"}), MaxAttempts(2)),
			func(ctx context.Context, Job Job) error {
				select {
				case <-firstTry:
					called <- "retry message"
					return nil
				default:
					firstTry <- struct{}{}
					return errors.New("some err")
				}
			},
			func() {
				<-called
				time.Sleep(100 * time.Millisecond)
				info, _ := consumer.driver.Info(context.Background())
				assert.Equal(t, int64(0), info.Failed)
			},
		},
		{
			"reload message",
			Adjust(Of(MockJob{Value: "hello"}), Timeout(time.Second)),
			func(ctx context.Context, Job Job) error {
				called <- "reload message"
				return errors.New("some err")
			},
			func() {
				<-called
				time.Sleep(100 * time.Millisecond)
				num, _ := consumer.driver.Reload(context.Background(), "failed")
				assert.Equal(t, int64(1), num)
				time.Sleep(5 * time.Millisecond)
				info, _ := consumer.driver.Info(context.Background())
				assert.Equal(t, int64(0), info.Failed)
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dispatcher := setUp()
			defer tearDown()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go dispatcher.Consume(ctx)
			go func() {
				dispatcher.Subscribe(c.ln)
				err := dispatcher.Dispatch(context.Background(), c.evt)
				assert.NoError(t, err)
			}()

			c.called()
		})
	}
}
