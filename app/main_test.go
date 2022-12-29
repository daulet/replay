package main_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"testing"

	"github.com/daulet/redisreplay"

	"github.com/go-redis/redis/v8"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
)

var redisPort string

func TestSimple(t *testing.T) {
	tests := []struct {
		name      string
		mode      redisreplay.Mode
		redisAddr string
	}{
		{
			name:      "record",
			mode:      redisreplay.ModeRecord,
			redisAddr: redisPort,
		},
		{
			name:      "replay",
			mode:      redisreplay.ModeReplay,
			redisAddr: ":1", // any address that doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				wg          sync.WaitGroup
				ctx, cancel = context.WithCancel(context.Background())
			)
			const port = 8081

			srv := redisreplay.NewRedisProxy(tt.mode, port, tt.redisAddr,
				func(reqID int) string {
					return fmt.Sprintf("testdata/%d.request", reqID)
				},
				func(reqID int) string {
					return fmt.Sprintf("testdata/%d.response", reqID)
				},
			)
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := srv.Serve(ctx); err != nil {
					log.Fatal(err)
				}
			}()

			rdb := redis.NewClient(&redis.Options{
				Addr:     fmt.Sprintf("localhost:%d", port),
				Password: "", // no password set
				DB:       0,  // use default DB
			})

			vals, err := rdb.Keys(ctx, "*").Result()
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, []string{}, vals)

			val, err := rdb.Set(ctx, "key", "value", 0).Result()
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, "OK", val)

			val, err = rdb.Get(ctx, "key").Result()
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, "value", val)

			vals, err = rdb.Keys(ctx, "*").Result()
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, []string{"key"}, vals)

			cancel()
			wg.Wait()
		})
	}
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("could not create pool: %s", err)
	}
	res, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "redis",
		Tag:        "6-alpine",
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("could not start resource: %s", err)
	}
	if err = pool.Retry(func() error {
		db := redis.NewClient(&redis.Options{
			Addr: res.GetHostPort("6379/tcp"),
		})
		return db.Ping(ctx).Err()
	}); err != nil {
		log.Fatalf("could not connect to redis: %s", err)
	}
	redisPort = res.GetHostPort("6379/tcp")
	exitCode := m.Run()
	if err := pool.Purge(res); err != nil {
		log.Fatalf("could not purge resource: %s", err)
	}
	os.Exit(exitCode)
}
