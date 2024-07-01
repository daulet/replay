package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func ServeApplication(ctx context.Context, port int, depAddr string) error {
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
		Handler: &application{
			dependencyAddr: depAddr,
		},
	}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

var _ http.Handler = &application{}

type application struct {
	dependencyAddr string
}

func (a *application) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	var newParts []string
	for _, part := range parts {
		i, err := strconv.Atoi(part)
		if err != nil {
			newParts = append(newParts, part)
			continue
		}
		newParts = append(newParts, strconv.Itoa(i*i))
	}

	resp, err := http.Get(fmt.Sprintf("http://%s%s", a.dependencyAddr, strings.Join(newParts, "/")))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	io.Copy(w, resp.Body)
}
