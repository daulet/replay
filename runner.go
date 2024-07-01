package replay

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"
)

type httpRunner struct {
	writeDir string

	// internal control
	done chan struct{}

	// internal state
	srv       *http.Server
	mux       sync.RWMutex
	requestID int
}

func NewHTTPRunner(port int, remoteAddr string, writeDir string) (*httpRunner, error) {
	srvMux := http.NewServeMux()
	runner := &httpRunner{
		writeDir: writeDir,
		done:     make(chan struct{}),
		srv: &http.Server{
			Addr:    fmt.Sprintf(":%v", port),
			Handler: srvMux,
		},
	}

	url, err := url.Parse(fmt.Sprintf("http://%s", remoteAddr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse remote address: [%w]", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(url)
	proxy.ModifyResponse = func(r *http.Response) error {
		return runner.recordResponse(r)
	}
	srvMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		runner.recordRequest(r)
		proxy.ServeHTTP(w, r)
	})
	srvMux.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		close(runner.done)
	})

	return runner, nil
}

func (h *httpRunner) Serve() error {
	go func() {
		<-h.done
		_ = h.srv.Shutdown(context.Background())
	}()
	if err := h.srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (h *httpRunner) recordRequest(r *http.Request) {
	fullReq, _ := httputil.DumpRequest(r, true)
	h.mux.RLock()
	f, err := os.OpenFile(fmt.Sprintf("%s/request%v.data", h.writeDir, h.requestID), os.O_CREATE|os.O_WRONLY, 0o644)
	h.mux.RUnlock()
	if err != nil {
		// h.log.Errorf("failed to create request file: %v", err)
	} else {
		defer f.Close()
		_, err = f.Write(fullReq)
		if err != nil {
			// h.log.Errorf("failed to write request file: %v", err)
		}
	}
}

func (h *httpRunner) recordResponse(resp *http.Response) error {
	if resp.Body == nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	h.mux.Lock()
	f, err := os.OpenFile(fmt.Sprintf("%s/response%v.data", h.writeDir, h.requestID), os.O_CREATE|os.O_WRONLY, 0o644)
	h.requestID += 1
	h.mux.Unlock()
	if err != nil {
		// h.log.Errorf("failed to create response file: %v", err)
		return err
	}
	defer f.Close()
	_, err = f.Write(body)
	if err != nil {
		// h.log.Errorf("failed to write response file: %v", err)
		return err
	}
	resp.Body = io.NopCloser(bytes.NewBuffer(body))
	return nil
}
