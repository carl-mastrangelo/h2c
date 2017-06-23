# h2c
HTTP/2 Cleartext Wrappers

```
$ go get github.com/carl-mastrangelo/h2c
```

By default, the Go HTTP Server only handles HTTP/2 connections over TLS as hinted by RFC 7540.
This is good for the security of the Internet, but makes things cumbersome when proxying requests
over an already secure connection (such as in-memory unit tests, Unix Domain Sockets, local TCP
connections, etc.).

The `h2c` package wraps an existing `http.Server`'s `Handler` in such a way that incoming 
cleartext requests are rewritten to be handled by the given HTTP/2 Server.  For example:

```go
	s := &http.Server{
		    Handler: grpc.NewServer(),
	}
	
	h2c.AttachClearTextHandler(nil /* default http2 server */, s)
	
	s.ListenAndServe()
``` 

For clients:

```go
	c := &http.Client{}
	h2c.AttachClearTextUpgrade(c)

	resp, err := c.Get("http://localhost:8080/")
``` 
