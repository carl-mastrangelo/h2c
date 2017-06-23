package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/carl-mastrangelo/h2c"
)

func main() {
	go startH1BackendServer()
	go startH2BackendServer()

	time.Sleep(500 * time.Millisecond)

	c := &http.Client{}
	h2c.AttachClearTextUpgrade(c)

	resp, err := c.Get("http://localhost:9902/")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(string(data))

	// Try again, with http 1

	resp, err = c.Get("http://localhost:9901/")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(string(data))
}

func startH2BackendServer() {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("Hello, World " + r.Proto)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	s := &http.Server{
		Addr:    ":9902",
		Handler: http.HandlerFunc(handler),
	}
	h2c.AttachClearTextHandler(nil, s)
	log.Fatal(s.ListenAndServe())
}

func startH1BackendServer() {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("Hello, World " + r.Proto)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	s := &http.Server{
		Addr:    ":9901",
		Handler: http.HandlerFunc(handler),
	}
	log.Fatal(s.ListenAndServe())
}
