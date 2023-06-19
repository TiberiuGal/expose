package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tiberiugal/expose"
)

func main() {
	u, err := url.Parse("/ws")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("u %+v", u)
	r := http.Request{}
	r.URL = u
	log.Printf("r %+v", r)
	log.Println("r requesturi", r.URL.RequestURI())
}

func mainWS() {

	conn, resp, err := websocket.DefaultDialer.Dial("ws://localhost:80/ws", nil)
	if err != nil {
		fmt.Println("error dialing", err)
		return
	}
	defer conn.Close()
	fmt.Println("got response", resp.Status)
	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error { conn.SetReadDeadline(time.Now().Add(60 * time.Second)); return nil })

	conn.WriteMessage(websocket.TextMessage, []byte("hello from client"))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		fmt.Println("error reading message", err)
		return
	}
	fmt.Println("got message", string(msg))

}
func main2() {
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
	req.Header.Add("X-WaitFor", "3s")
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
	if wf := r.Header.Get("X-WaitFor"); wf != "" {
		fmt.Println("waiting for", wf)
		d, _ := time.ParseDuration(wf)
		time.Sleep(d)
	}
	fmt.Fprint(w, "lorem ipsum", h.cnt)
	fmt.Fprintln(w, r.RequestURI)

}
