package queue

import (
	"context"
	"testing"
	"time"

	"github.com/DoNewsCode/core/config"
	"github.com/DoNewsCode/core/di"
	"github.com/DoNewsCode/core/otredis"
	"github.com/go-kit/kit/log"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

type maker struct{}

func (m maker) Make(name string) (redis.UniversalClient, error) {
	envDefaultRedisAddrs, _ := getDefaultRedisAddrs()
	return redis.NewUniversalClient(&redis.UniversalOptions{Addrs: envDefaultRedisAddrs}), nil
}

type populator struct{}

func (p populator) Populate(target interface{}) error {
	*(target.(*otredis.Maker)) = maker{}
	return nil
}

func TestProvideDispatcher(t *testing.T) {
	out, err := provideDispatcherFactory(&providersOption{})(makerIn{
		Conf: config.WithAccessor(config.MapAdapter{"queue": map[string]configuration{
			"default": {
				"default",
				1,
				5,
			},
			"alternative": {
				"default",
				3,
				5,
			},
		}}),
		JobDispatcher: &SyncDispatcher{},
		Populator:     populator{},
		Logger:        log.NewNopLogger(),
		AppName:       config.AppName("test"),
		Env:           config.EnvTesting,
	})
	assert.NoError(t, err)
	assert.NotNil(t, out.DispatcherFactory)
	def, err := out.DispatcherFactory.Make("alternative")
	assert.NoError(t, err)
	assert.NotNil(t, def)
	assert.Implements(t, (*di.Modular)(nil), out)
}

type mockDriver struct {
}

func (m mockDriver) Push(ctx context.Context, message *PersistedJob, delay time.Duration) error {
	panic("implement me")
}

func (m mockDriver) Pop(ctx context.Context) (*PersistedJob, error) {
	panic("implement me")
}

func (m mockDriver) Ack(ctx context.Context, message *PersistedJob) error {
	panic("implement me")
}

func (m mockDriver) Fail(ctx context.Context, message *PersistedJob) error {
	panic("implement me")
}

func (m mockDriver) Reload(ctx context.Context, channel string) (int64, error) {
	panic("implement me")
}

func (m mockDriver) Flush(ctx context.Context, channel string) error {
	panic("implement me")
}

func (m mockDriver) Info(ctx context.Context) (QueueInfo, error) {
	panic("implement me")
}

func (m mockDriver) Retry(ctx context.Context, message *PersistedJob) error {
	panic("implement me")
}

func TestProvideDispatcher_withDriver(t *testing.T) {
	out, err := provideDispatcherFactory(&providersOption{driver: mockDriver{}})(makerIn{
		Conf: config.WithAccessor(config.MapAdapter{"queue": map[string]configuration{
			"default": {
				"default",
				1,
				5,
			},
			"alternative": {
				"default",
				3,
				5,
			},
		}}),
		JobDispatcher: &SyncDispatcher{},
		Logger:        log.NewNopLogger(),
		AppName:       config.AppName("test"),
		Env:           config.EnvTesting,
	})
	assert.NoError(t, err)
	assert.NotNil(t, out.DispatcherFactory)
	def, err := out.DispatcherFactory.Make("alternative")
	assert.NoError(t, err)
	assert.NotNil(t, def)
	assert.Implements(t, (*di.Modular)(nil), out)
}

func TestProvideConfigs(t *testing.T) {
	c := provideConfig()
	assert.NotEmpty(t, c.Config)
}
