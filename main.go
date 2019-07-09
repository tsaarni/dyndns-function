package main

import (
	"fmt"
	"net/http"

	"github.com/tsaarni/dyndns-function/functions"
)

var listen = "127.0.0.1:8080"

func main() {
	http.HandleFunc("/Update", functions.Update)
	fmt.Printf("Listening for incoming requests http://%s\n", listen)
	http.ListenAndServe(listen, nil)
}
