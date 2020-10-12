package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
 	h := &handler{}
	http.ListenAndServe(":8022", h)
}


type handler struct {
	cnt int
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.cnt++
	w.WriteHeader(http.StatusOK)
	log.Println("received request", r.Host, r.RequestURI, r.Header)
	io.Copy(os.Stdout, r.Body)
	fmt.Fprint(w, "lorem ipsum", h.cnt)
	fmt.Fprintln(w, r.RequestURI)
}
