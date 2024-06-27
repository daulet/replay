package main

import (
	"context"
	"fmt"
	"html"
	"net/http"
)

func Serve(ctx context.Context, port int) error {
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
	}

	http.HandleFunc("/foo", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
	})

	http.HandleFunc("/bar", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hi, %q", html.EscapeString(r.URL.Path))
	})

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}
