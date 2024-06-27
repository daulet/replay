package main

import (
	"fmt"
	"html"
	"net/http"
)

func serve(port int) error {
	http.HandleFunc("/foo", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
	})

	http.HandleFunc("/bar", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hi, %q", html.EscapeString(r.URL.Path))
	})

	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
