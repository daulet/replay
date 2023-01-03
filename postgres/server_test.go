package postgres_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"io"
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
	"go.uber.org/zap"
)

var dbPort int

const (
	host     = "localhost"
	dbname   = "testdb"
	username = "testuser"
	password = "testpassword"
)

/*
The first byte of a message identifies the message type,
and the next four bytes specify the length of the rest of the message
(this length count includes itself, but not the message-type byte).
The remaining contents of the message are determined by the message type.
For historical reasons, the very first message sent by the client
(the startup message) has no initial message-type byte.
*/
func TestParse(t *testing.T) {
	const request = true
	f, err := os.Open("testdata/ingress")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	bs, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}

	i := 0
	fmt.Printf("total len: %d\n", len(bs))

	if request {
		/*
		 Startup phase
		*/
		fmt.Printf("[length]: %v\n", binary.BigEndian.Uint32(bs[i:i+4]))
		i += 4
		// close-complete
		fmt.Printf("[message]: %v\n", bs[i:i+4])
		i += 4
		// key-value pairs
		for bs[i] != 0 { // can't start with NUL
			j := i
			for j = i; bs[j] != 0; j++ {
				// search for NUL terminated end of string
			}
			fmt.Printf("[string]: %s ([len=%d]%v)\n", bs[i:j], len(bs[i:j]), bs[i:j])
			i = j + 1
		}
		i += 1 // skip the NUL
	}

	/*
		Normal phase
	*/
	// Parse messages: https://www.postgresql.org/docs/current/protocol-message-formats.html
	for i < len(bs) {
		fmt.Printf("[message]: %d ('%c')\n", bs[i], bs[i])
		if bs[i] == 'X' { // Terminate
			break
		}
		i += 1

		length := int(binary.BigEndian.Uint32(bs[i : i+4]))
		fmt.Printf("[length]: %d\n", length)

		// length value includes itself and NULL terminator
		value := bs[i+4 : i+length]
		for _, part := range bytes.Split(value, []byte{0}) {
			if len(part) == 0 {
				continue
			}
			fmt.Printf("%s ([len=%d]%v)\n", part, len(part), part)
		}
		i += length
	}
}

func TestSimple(t *testing.T) {
	const port = 5555

	logger, err := zap.NewProduction()
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Sync()

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
		if err := thru.Serve(ctx, port, fmt.Sprintf("localhost:%d", port+1), in, out); err != nil {
			log.Fatal(err)
		}
	}()

	srv := redisreplay.NewRedisProxy(redisreplay.ModeRecord, port+1, fmt.Sprintf("localhost:%d", dbPort),
		func(reqID int) string {
			return fmt.Sprintf("testdata/%d.request", reqID)
		},
		func(reqID int) string {
			return fmt.Sprintf("testdata/%d.response", reqID)
		},
		redisreplay.ProxyLogger(logger),
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.Serve(ctx); err != nil {
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
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS test (id int)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO test VALUES (1)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO test VALUES (10)"); err != nil {
		t.Fatal(err)
	}
	if rows, err := db.QueryContext(ctx, "SELECT * FROM test"); err != nil {
		t.Fatal(err)
	} else {
		fmt.Println("Rows:")
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			fmt.Println(id)
		}
		rows.Close()
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
			fmt.Sprintf("POSTGRES_HOST_AUTH_METHOD=%s", "trust"),
			// this is not necessary when using trust auth method
			// however if the line above is removed this will be useful to be setup
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
