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
)

// TODO test should use redis client to generate calls, passthrough for recording, retest on replay

var redisPort string

func TestSimple(t *testing.T) {
	var (
		wg          sync.WaitGroup
		ctx, cancel = context.WithCancel(context.Background())
	)
	const port = 8081

	srv := redisreplay.NewRedisProxy(port, redisPort)
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

	if err := rdb.Keys(ctx, "*").Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Set(ctx, "key", "value", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Get(ctx, "key").Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Keys(ctx, "*").Err(); err != nil {
		t.Fatal(err)
	}

	cancel()
	wg.Wait()
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
