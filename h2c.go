package h2c

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
	"sync"

	"golang.org/x/net/http2"
)

type h2ForwardConn struct {
	net.Conn

	pref *bytes.Buffer
	mu   *sync.Mutex
	r    *bufio.Reader
}

func newH2ForwardConn(c net.Conn, rw *bufio.ReadWriter) net.Conn {
	if rw.Writer.Buffered() != 0 {
		panic("buffered writer data") // This should never happen, according to net/http's Hijacker
	}
	return &h2ForwardConn{
		mu:   new(sync.Mutex),
		Conn: c,
		pref: bytes.NewBufferString(http2.ClientPreface[0:18]),
		r:    rw.Reader,
	}
}

func (c *h2ForwardConn) Read(b []byte) (n int, err error) {
	c.mu.Lock()
	if c.pref.Len() != 0 {
		defer c.mu.Unlock()
		return c.pref.Read(b)
	}
	if c.r.Buffered() > 0 {
		defer c.mu.Unlock()
		return c.r.Read(b)
	}
	c.mu.Unlock()
	return c.Conn.Read(b)
}

type h2FowardHandler struct {
	delegate http.Handler
	h2s      *http2.Server
	s        *http.Server
}

func NewClearTextHandler(h2s *http2.Server, s *http.Server, delegate http.Handler) http.Handler {
	if h2s == nil {
		h2s = &http2.Server{}
	}
	return &h2FowardHandler{
		h2s:      h2s,
		s:        s,
		delegate: delegate,
	}
}

func AttachClearTextHandler(h2s *http2.Server, s *http.Server) {
	s.Handler = NewClearTextHandler(h2s, s, s.Handler)
}

func (h *h2FowardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "PRI" && r.URL.String() == "*" && r.Proto == "HTTP/2.0" {
		if hj, ok := w.(http.Hijacker); ok {
			conn, rw, err := hj.Hijack()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer conn.Close()
			h.h2s.ServeConn(newH2ForwardConn(conn, rw), &http2.ServeConnOpts{
				BaseConfig: h.s,
			})
			return
		}
	}
	h.delegate.ServeHTTP(w, r)
}
