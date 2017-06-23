package h2c

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"log"
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
	if up := r.Header.Get("Upgrade"); up == "h2c" {
		w.WriteHeader(http.StatusSwitchingProtocols)
		return
	}
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

type upgrader struct {
	h1   http.RoundTripper
	h2   http.RoundTripper
	mu   *sync.Mutex
	pref map[string]http.RoundTripper
}

func (u *upgrader) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := func() error {
		u.mu.Lock()
		defer u.mu.Unlock()
		if u.pref[req.URL.Host] == nil {
			upreq, err := http.NewRequest("OPTIONS", "*", nil)
			if err != nil {
				return err
			}
			upreq.URL.Scheme = req.URL.Scheme
			upreq.URL.Host = req.URL.Host
			upreq.URL.Path = req.URL.Path
			upreq.Header.Add("Upgrade", "h2c")
			upreq.Header.Add("Connection", "close") // don't risk the pooled connection living
			upresp, err := u.h1.RoundTrip(upreq)
			if err != nil {
				return err
			}
			defer upresp.Body.Close()

			// TODO: clear out u.pref once in a while
			if upresp.StatusCode == http.StatusSwitchingProtocols {
				u.pref[req.URL.Host] = u.h2
			} else {
				u.pref[req.URL.Host] = u.h1
			}
		}
		return nil
	}(); err != nil {
		log.Println("uh oh", err)
		return nil, err
	}

	u.mu.Lock()
	rt := u.pref[req.URL.Host]
	u.mu.Unlock()

	return rt.RoundTrip(req)
}

func AttachClearTextUpgrade(c *http.Client) {
	var h1 http.RoundTripper
	if c.Transport != nil {
		h1 = c.Transport
	} else {
		h1 = http.DefaultTransport
	}

	h2 := &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			ta, err := net.ResolveTCPAddr(network, addr)
			if err != nil {
				return nil, err
			}
			return net.DialTCP(network, nil, ta)
		},
	}

	c.Transport = &upgrader{
		h1:   h1,
		h2:   h2,
		mu:   new(sync.Mutex),
		pref: make(map[string]http.RoundTripper),
	}
}
