package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"

	"github.com/tsaarni/dyndns"
)

var listen = "127.0.0.1:8080"

type LoggingTransport struct {
	Transport http.RoundTripper
}

func debug(data []byte, err error) {
	if err == nil {
		fmt.Printf("%s\n\n", data)
	} else {
		log.Fatalf("%s\n\n", err)
	}
}

func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	debug(httputil.DumpRequestOut(req, true))

	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	debug(httputil.DumpResponse(resp, true))

	return resp, err
}

func main() {
	http.DefaultTransport = &LoggingTransport{
		Transport: http.DefaultTransport,
	}

	http.HandleFunc("/Update", dyndns.Update)
	fmt.Printf("Listening for incoming requests http://%s\n", listen)
	http.ListenAndServe(listen, nil)
}
