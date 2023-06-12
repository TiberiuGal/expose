package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/tiberiugal/expose"
)

func main() {
	h := &handler{}

	go func() {
		http.ListenAndServe(":8022", h)
	}()
	cloudServer := expose.NewCloudServer()
	ln, err := net.Listen("tcp", ":1044")
	if err != nil {
		log.Fatal(err)
	}
	waifForConnection := make(chan struct{})
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Println("error accepting connection", err)
				continue
			}
			err = cloudServer.AcceptEdgeConnection(conn)
			if err != nil {
				log.Println("error accepting connection", err)
				continue
			}
			waifForConnection <- struct{}{}
			log.Println("new connection accepted", conn.RemoteAddr())
		}
	}()

	cloudConn, err := net.Dial("tcp", "localhost:1044")
	if err != nil {
		log.Fatal(err)
	}

	edgeServer := expose.NewEdgeServer("localhost:8022", "tibi", cloudConn)
	go edgeServer.Run()

	rw := &responseWriter{header: make(http.Header)}
	req, _ := http.NewRequest("GET", "/lorem", nil)
	req.Host = "tibi"
	<-waifForConnection
	cloudServer.ServeHTTP(rw, req)
	fmt.Println(rw.status)
	fmt.Println(string(rw.body))
}

type responseWriter struct {
	header http.Header
	status int
	body   []byte
}

func (rw *responseWriter) Header() http.Header {
	return rw.header
}

func (rw *responseWriter) Write(p []byte) (int, error) {
	rw.body = p
	return len(p), nil
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.status = statusCode
}

func (rw *responseWriter) Close() error {
	fmt.Println("closing")
	return nil
}

func (rw *responseWriter) Flush() {
	fmt.Println("flushing")
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
