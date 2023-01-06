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
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/daulet/replay"
	"github.com/daulet/replay/postgres"

	_ "github.com/lib/pq"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
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
	t.Skip()
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
		m := make(map[string]string)
		for bs[i] != 0 { // can't start with NUL
			var ss []string
			for k := 0; k < 2; k++ {
				j := i
				for j = i; bs[j] != 0; j++ {
					// search for NUL terminated end of string
				}
				ss = append(ss, string(bs[i:j]))
				i = j + 1
			}
			m[ss[0]] = ss[1]
		}
		i += 1 // skip the NUL

		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("%s => %s\n", k, m[k])
		}
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
		fmt.Printf("%s ([len=%d]%v)\n", value, len(value), value)
		for _, part := range bytes.Split(value, []byte{0}) {
			if len(part) == 0 {
				continue
			}
			fmt.Printf("%s ([len=%d]%v)\n", part, len(part), part)
		}
		i += length
	}
}

func TestPostgres(t *testing.T) {
	const port = 5555

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
				return postgres.NewRecorder(fmt.Sprintf("localhost:%d", dbPort), reqFunc, resFunc)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				wg          sync.WaitGroup
				ctx, cancel = context.WithCancel(context.Background())
			)

			thru := replay.NewPassthrough()
			wg.Add(1)
			go func() {
				defer wg.Done()
				// TODO these outputs are not stable
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

			rw, err := tt.rwFunc()
			if err != nil {
				t.Fatal(err)
			}

			srv := replay.NewProxy(port+1, rw, replay.ProxyLogger(logger))
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

			res, err := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS test (id int)")
			if err != nil {
				t.Fatal(err)
			}
			affected, err := res.RowsAffected()
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, int64(0), affected)

			res, err = db.ExecContext(ctx, "INSERT INTO test VALUES (1)")
			if err != nil {
				t.Fatal(err)
			}
			affected, err = res.RowsAffected()
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, int64(1), affected)

			res, err = db.ExecContext(ctx, "INSERT INTO test VALUES (10)")
			if err != nil {
				t.Fatal(err)
			}
			affected, err = res.RowsAffected()
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, int64(1), affected)

			rows, err := db.QueryContext(ctx, "SELECT * FROM test")
			if err != nil {
				t.Fatal(err)
			}
			var vals []int
			for rows.Next() {
				var id int
				if err := rows.Scan(&id); err != nil {
					t.Fatal(err)
				}
				vals = append(vals, id)
			}
			rows.Close()
			assert.Equal(t, []int{1, 10}, vals)

			// reset state so tests can be run multiple times
			res, err = db.ExecContext(ctx, "DROP TABLE test")
			if err != nil {
				t.Fatal(err)
			}
			affected, err = res.RowsAffected()
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, int64(0), affected)

			db.Close() // close connection to proxy
			cancel()   // stop proxy
			// TODO close rw
			// rw.Close() // close connection to read/writer
			wg.Wait() // wait for proxy to stop
		})
	}
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
	res.Close()
	os.Exit(exitCode)
}
