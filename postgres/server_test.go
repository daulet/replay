package postgres_test

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

var dbPort string

// psql -h localhost -p 55198 -U testuser testdb
// Password: testpassword
func TestMain(m *testing.M) {
	const (
		dbname   = "testdb"
		username = "testuser"
		password = "testpassword"
	)

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

	dbPort = res.GetHostPort("5432/tcp")
	connStr := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
		username, password, dbPort, dbname)

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
