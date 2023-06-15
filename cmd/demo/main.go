package main

import (
	"fmt"
	"net/http"
)

func main() {
	srv := &server{}
	http.ListenAndServe(":80", srv)
}

type server struct {
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "hello")
	return
}
