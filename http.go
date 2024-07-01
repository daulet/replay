package replay

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/google/go-replayers/httpreplay"
)

var _ io.Closer = (*HTTPServer)(nil)

type HTTPServer struct {
	// internal state
	wg  *sync.WaitGroup
	srv *http.Server
	r   *httpreplay.Recorder
}

// TODO strongly typed params for URL and Path
// TODO perhaps Serving part should be separate from the constructor
func NewHTTPServer(port int, remoteAddr string, recordFile string) (*HTTPServer, error) {
	{
		f, err := os.OpenFile(recordFile, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to create record file: [%w]", err)
		}
		f.Close()
	}
	// TODO switch based on mode
	rep, err := httpreplay.NewRecorderWithOpts(recordFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open record file: [%w]", err)
	}

	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
		Handler: &httpHandler{
			remoteAddr: remoteAddr,
			client:     rep.Client(),
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.ListenAndServe()
	}()

	return &HTTPServer{
		wg:  &wg,
		srv: srv,
		r:   rep,
	}, nil
}

func (h *HTTPServer) Close() error {
	err := h.srv.Shutdown(context.Background())
	h.r.Close()
	h.wg.Wait()
	return err
}

var _ http.Handler = (*httpHandler)(nil)

type httpHandler struct {
	remoteAddr string
	client     *http.Client
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.RequestURI = ""
	u, err := url.Parse(fmt.Sprintf("http://%s%s", h.remoteAddr, r.URL.Path))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	r.URL = u
	r.Host = u.Host
	resp, err := h.client.Do(r)
	if err != nil {
		w.WriteHeader(resp.StatusCode)
		w.Write([]byte(err.Error()))
		return
	}
	defer resp.Body.Close()
	if _, err := io.Copy(w, resp.Body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
}
