package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/daulet/redisreplay"

	_ "github.com/lib/pq"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

var dbPort int

const (
	host     = "localhost"
	dbname   = "testdb"
	username = "testuser"
	password = "testpassword"
)

func TestSimple(t *testing.T) {
	const port = 5555

	var (
		wg          sync.WaitGroup
		ctx, cancel = context.WithCancel(context.Background())
	)

	thru := redisreplay.NewPassthrough()
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
		if err := thru.Serve(ctx, port, fmt.Sprintf("localhost:%d", dbPort), in, out); err != nil {
			log.Fatal(err)
		}
	}()

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		username, password, host, port, dbname)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}

	db.Close() // close connection to proxy
	cancel()   // stop proxy
	wg.Wait()  // wait for proxy to stop
}

// psql -h localhost -p 55198 -U testuser testdb
// Password: testpassword
func TestMain(m *testing.M) {

	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("could not create pool: %s", err)
	}
	res, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15",
		Env: []string{
			fmt.Sprintf("POSTGRES_PASSWORD=%s", password),
			fmt.Sprintf("POSTGRES_USER=%s", username),
			fmt.Sprintf("POSTGRES_DB=%s", dbname),
			"listen_addresses = '*'",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("could not start resource: %s", err)
	}
	defer res.Close()

	{
		portStr := res.GetPort("5432/tcp")
		dbPort, err = strconv.Atoi(portStr)
		if err != nil {
			log.Fatalf("could not parse port: %s", err)
		}
	}
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		username, password, host, dbPort, dbname)

	pool.MaxWait = 10 * time.Second
	if err = pool.Retry(func() error {
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			return err
		}
		defer db.Close()
		return db.Ping()
	}); err != nil {
		log.Fatalf("could not connect to postgres: %s", err)
	}
	exitCode := m.Run()
	if err := pool.Purge(res); err != nil {
		log.Fatalf("could not purge resource: %s", err)
	}
	os.Exit(exitCode)
}
