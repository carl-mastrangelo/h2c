package main

import (
	"crypto/tls"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"

	"golang.org/x/net/http2"

	"github.com/carl-mastrangelo/h2c"
)

func main() {
	go startBackendServer()

	go startProxy()

	resp, err := http.Get("http://localhost:8800/")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(string(data))
}

func startBackendServer() {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("Hello, World " + r.Proto)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	s := &http.Server{
		Addr:    ":9900",
		Handler: http.HandlerFunc(handler),
	}
	h2c.AttachClearTextHandler(nil, s)
	log.Fatal(s.ListenAndServe())
}

func startProxy() {
	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "localhost:9900"
		},
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				ta, err := net.ResolveTCPAddr(network, addr)
				if err != nil {
					return nil, err
				}
				return net.DialTCP(network, nil, ta)
			},
		},
	}

	log.Fatal(http.ListenAndServe(":8800", rp))
}
