package main

import (
	"crypto/tls"
	"io/ioutil"
	"log"
	"net"
	"net/http"

	"golang.org/x/net/http2"

	"github.com/carl-mastrangelo/h2c"
)

func main() {
	startServer()

	t := &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			ua, err := net.ResolveUnixAddr("unix", "./server.socket")
			if err != nil {
				return nil, err
			}
			return net.DialUnix("unix", nil, ua)
		},
	}

	client := http.Client{
		Transport: t,
	}

	resp, err := client.Get("http://server.socket/")
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

func startServer() {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("Hello, World " + r.Proto)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	s := &http.Server{
		Handler: http.HandlerFunc(handler),
	}
	h2c.AttachClearTextHandler(nil, s)

	ua, err := net.ResolveUnixAddr("unix", "./server.socket")
	if err != nil {
		log.Fatal(err)
	}

	ln, err := net.ListenUnix("unix", ua)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		defer ln.Close()
		log.Fatal(s.Serve(ln))
	}()
}
