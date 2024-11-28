package grpc_test

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"syscall"
	"testing"
	"time"

	example "examples/grpc"
	pb "examples/grpc/helloworld"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const TEST_TIMEOUT = 2 * time.Second

func TestHelloWorld(t *testing.T) {
	const port = 50051

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), TEST_TIMEOUT)
	defer cancel()

	lcfg := &net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			})
		},
	}
	lis, err := lcfg.Listen(ctx, "tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterGreeterServer(s, &example.Server{})

	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// act & assert
	{
		conn, err := grpc.NewClient(fmt.Sprintf("localhost:%d", port), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("did not connect: %v", err)
		}
		defer conn.Close()
		c := pb.NewGreeterClient(conn)

		tests := []struct {
			name string
			req  *pb.HelloRequest
			want string
		}{
			{name: "world", req: &pb.HelloRequest{Name: "world"}, want: "Hello world"},
			{name: "empty", req: &pb.HelloRequest{}, want: "Hello "},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				resp, err := c.SayHello(ctx, test.req)
				if err != nil {
					log.Fatalf("could not greet: %v", err)
				}
				require.Equal(t, test.want, resp.GetMessage())
			})
		}
	}

	cancel()
	wg.Wait()
}
