package replay

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/go-cmp/cmp"
)

type httpRunner struct {
	remoteAddr string
	writeDir   string

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
		remoteAddr: fmt.Sprintf("http://%s", remoteAddr),
		writeDir:   writeDir,
		done:       make(chan struct{}),
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
	// TODO replace reverse proxy as the current implementation only provides callback
	// without original request. Could be as simple as implementing a custom http.ResponseWriter
	// and pass it to ServeHTTP in srvMux.HandleFunc("/", ...)
	proxy.ModifyResponse = func(r *http.Response) error {
		return runner.recordResponse(r, nil)
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		runner.recordResponse(nil, err)
		w.WriteHeader(http.StatusBadGateway)
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

type indexedResponse struct {
	index int
	resp  *http.Response
	err   error
}

func (h *httpRunner) Replay(updateResponses bool) error {
	var (
		reqs      []*http.Request
		wantResps [][]byte
	)
	for i := 0; ; i++ {
		reqPath := filepath.Join(h.writeDir, fmt.Sprintf("request%v.data", i))
		f, err := os.Open(reqPath)
		if err != nil {
			break
		}
		req, err := http.ReadRequest(bufio.NewReader(f))
		if err != nil {
			return fmt.Errorf("failed to read request from file %q: [%w]", reqPath, err)
		}
		reqs = append(reqs, req)

		respPath := filepath.Join(h.writeDir, fmt.Sprintf("response%v.data", i))
		b, err := os.ReadFile(respPath)
		if err != nil {
			return fmt.Errorf("failed to read response from file %q: [%w]", respPath, err)
		}
		wantResps = append(wantResps, b)
	}
	resps := make([]*http.Response, len(reqs))
	{
		respCh := make(chan indexedResponse)
		var wg sync.WaitGroup
		for i, req := range reqs {
			wg.Add(1)
			go func(i int, req *http.Request) {
				defer wg.Done()

				req.RequestURI = ""
				u, err := url.Parse(fmt.Sprintf("%s%s", h.remoteAddr, req.URL.Path))
				if err != nil {
					respCh <- indexedResponse{index: i, err: err}
					return
				}
				req.URL = u
				resp, err := http.DefaultClient.Do(req)
				respCh <- indexedResponse{i, resp, err}
			}(i, req)
		}
		for range reqs {
			resp := <-respCh
			// TODO deal with error
			resps[resp.index] = resp.resp
		}
		wg.Wait()
		close(respCh)
	}
	for i, resp := range resps {
		var rawResp []byte
		var err error
		if resp != nil {
			// remove Date header as it's not deterministic
			resp.Header.Del("Date")
			rawResp, err = httputil.DumpResponse(resp, true)
			if err != nil {
				return fmt.Errorf("failed to dump response: [%w]", err)
			}
		}
		if updateResponses {
			err := os.WriteFile(filepath.Join(h.writeDir, fmt.Sprintf("response%v.data", i)), rawResp, 0o644)
			if err != nil {
				return fmt.Errorf("failed to update response file: [%w]", err)
			}
			continue
		}
		wantResp := wantResps[i]
		if diff := cmp.Diff(string(wantResp), string(rawResp)); diff != "" {
			return fmt.Errorf("%d-th HTTP response diff: (-got +want)\n%s", i, diff)
		}
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

func (h *httpRunner) recordResponse(resp *http.Response, respErr error) error {
	h.mux.Lock()
	f, err := os.OpenFile(fmt.Sprintf("%s/response%v.data", h.writeDir, h.requestID), os.O_CREATE|os.O_WRONLY, 0o644)
	h.requestID += 1
	h.mux.Unlock()
	if err != nil {
		// h.log.Errorf("failed to create response file: %v", err)
		return err
	}
	defer f.Close()

	if respErr != nil {
		f.Write([]byte(respErr.Error()))
		return nil
	}

	// remove Date header as it's not deterministic
	resp.Header.Del("Date")
	rawResp, err := httputil.DumpResponse(resp, true)
	if err != nil {
		return fmt.Errorf("failed to dump response: [%w]", err)
	}
	_, err = f.Write(rawResp)
	if err != nil {
		return fmt.Errorf("failed to write response file: [%w]", err)
	}
	return nil
}
