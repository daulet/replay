package redis_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"testing"

	"github.com/daulet/replay"
	"github.com/daulet/replay/redis"

	redisv8 "github.com/go-redis/redis/v8"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

var redisPort string

func TestSimple(t *testing.T) {
	logger, err := zap.NewProduction()
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Sync()

	reqFunc := func(reqID int) string {
		return fmt.Sprintf("testdata/%d.request", reqID)
	}
	resFunc := func(reqID int) string {
		return fmt.Sprintf("testdata/%d.response", reqID)
	}

	tests := []struct {
		name   string
		rwFunc func() (io.ReadWriteCloser, error)
	}{
		{
			name: "record",
			rwFunc: func() (io.ReadWriteCloser, error) {
				return replay.NewRecorder(redisPort, reqFunc, resFunc)
			},
		},
		{
			name: "replay",
			rwFunc: func() (io.ReadWriteCloser, error) {
				return redis.NewReplayer(reqFunc, resFunc, redis.ReplayerLogger(logger))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				wg          sync.WaitGroup
				ctx, cancel = context.WithCancel(context.Background())
			)
			const port = 8081

			rw, err := tt.rwFunc()
			if err != nil {
				t.Fatal(err)
			}

			thru := replay.NewPassthrough()
			wg.Add(1)
			go func() {
				defer wg.Done()
				in, err := os.Create("testdata/ingress")
				if err != nil {
					log.Fatal(err)
				}
				defer in.Close()
				out, err := os.Create("testdata/egress")
				if err != nil {
					log.Fatal(err)
				}
				defer out.Close()
				if err := thru.Serve(ctx, port, fmt.Sprintf("localhost:%d", port+1), in, out); err != nil {
					log.Fatal(err)
				}
			}()

			srv := replay.NewProxy(port+1, rw, replay.ProxyLogger(logger))
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := srv.Serve(ctx); err != nil {
					log.Fatal(err)
				}
			}()

			rdb := redisv8.NewClient(&redisv8.Options{
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

			// reset state so tests can be run multiple times
			if err := rdb.FlushDB(ctx).Err(); err != nil {
				t.Fatal(err)
			}
			rw.Close()  // close connection to read/writer
			rdb.Close() // close connection to proxy
			cancel()    // stop proxy
			wg.Wait()   // wait for proxy to stop
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
	defer res.Close()
	if err = pool.Retry(func() error {
		db := redisv8.NewClient(&redisv8.Options{
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
