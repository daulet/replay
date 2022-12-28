package main_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"testing"
	"time"

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

	var lstr *net.TCPListener
	{
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()

		lstr = l.(*net.TCPListener)
	}
	var remote *net.TCPConn
	{
		conn, err := net.Dial("tcp", redisPort)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		remote = conn.(*net.TCPConn)
	}
	// TODO this should be a distinct service
	dst := redisreplay.NewRecorder(
		remote,
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
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			lstr.SetDeadline(time.Now().Add(1 * time.Second))
			src, err := lstr.Accept()
			if err != nil {
				if ne, ok := err.(net.Error); ok && !ne.Timeout() {
					t.Error(err)
				}
				continue
			}
			go handle(src, dst)
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

func handle(src io.ReadWriteCloser, dst io.ReadWriter) {
	defer src.Close()

	go func() {
		if _, err := io.Copy(dst, src); err != nil {
			log.Printf("write from in to out: %v", err)
		}
	}()
	if _, err := io.Copy(src, dst); err != nil {
		log.Printf("write from out to in: %v", err)
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
