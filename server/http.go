package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type proxy struct {
	clients map[string]*client
	m    sync.Mutex
}

type client struct {
	name string
	addr string
	pass string
	conn net.Conn
	m    sync.Mutex
}


func serve(ctx context.Context, addr string, wg *sync.WaitGroup, handler http.Handler) {
	defer wg.Done()

	server := &http.Server{Addr: addr, Handler: handler}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Println("failed to start the server", err)
		}
	}()

	<-ctx.Done()
	log.Println("context is closed, shutting down")
	server.Shutdown(context.TODO())

}

func newProxy() *proxy {
	return &proxy{
		clients: map[string]*client{},
	}
}

func (p *proxy) ListenForClients(port string) {
	l, err := net.Listen("tcp4", port)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer l.Close()

	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		log.Println("new connection incoming")
		go p.handleConnection(c)

	}
}

type temporary interface {
	Temporary()bool
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("received request", r.RequestURI, r.Host)

	parts := strings.Split(r.Host, ".")
	host := parts[0]
	p.m.Lock()
	cli, ok := p.clients[ host ]
	p.m.Unlock()
	if !ok {
		w.WriteHeader(404)
		fmt.Fprintln(w, "client does not exist")
		return
	}

	err := cli.timedHandler(2 * time.Second, w, r)
	if err != nil {
		log.Println("error handling", err)
		if e, ok := err.(temporary) ; ok && e.Temporary() {
			//retry?
			log.Println("temporary error", e)

		} else {
			log.Println("permanent error?, disconnect")
			cli.conn.Close()
			p.m.Lock()
			delete(p.clients, host )
			p.m.Unlock()
		}
	}
}

func (p *proxy) handleConnection(c net.Conn) {

	var name, pass string
	n, err := fmt.Fscanf(c, "%s %s\n", &name, &pass)
	if err != nil {
		fmt.Println("error reading string from client", err, n)
		c.Close()
		return
	}
	log.Println("received n whatever from fscanf", n)

	if name == "" {
		name = fmt.Sprintf("%d", len(p.clients)+1)
	}
	p.m.Lock()
	cli, ok := p.clients[name]
	p.m.Unlock()
	if ok {
		if cli.pass != cli.pass {
			fmt.Fprintln(c, "invalid pass")
			c.Close()
			return
		}
	}

	if pass == "" {
		pass = "this-is-random"
	}
	log.Println("accepted name+pass", name, pass)
	fmt.Fprintf(c, "%s %s\n", name, pass)

	netData, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		log.Println("error reading string from client", err)
		c.Close()
		return
	}
	log.Println("registerning", name)
	if strings.TrimSpace(string(netData)) != "ready" {

		fmt.Fprintln(c, "byte then")
		log.Println("client failed to ack")
		c.Close()
		return
	}
	p.m.Lock()
	p.clients[name] = &client{conn: c, name: name, pass: pass, addr: c.LocalAddr().String()}
	p.m.Unlock()
}

func (c *client) timedHandler(d time.Duration, w http.ResponseWriter, r *http.Request ) error {
		responseChan := make(chan struct{}, 1)
		var err error
		go func() {
			err = c.Handle(w, r)
			responseChan <- struct{}{}
		}()

		select {
			case <- time.After(2 * time.Second):
				log.Println("timeout exceeded on handler")
				return invalidResponseError
			case <-responseChan:
				log.Println("got completion notification on response chan")
				return err

		}
		return err

}
func (c *client) Handle(w http.ResponseWriter, r *http.Request) error {
	c.m.Lock()
	defer c.m.Unlock()
	// extract header
	hbr := bytes.NewBuffer([]byte{})
	r.Header.Write(hbr)
	headerSize := hbr.Len()

	bbr := bytes.NewBuffer([]byte{})
	io.Copy(bbr, r.Body)
	bodySize := bbr.Len()
	// init request to client
	log.Println("new request: header ", hbr.String())
	fmt.Fprintf(c.conn, "%s %s %d %d ", r.Method, r.RequestURI, headerSize, bodySize)
	//send request to client
	io.Copy(c.conn, hbr)
	io.Copy(c.conn, bbr)

	var status, hl, bl int
	//read response from client
	fmt.Fscanf(c.conn, "%d %d %d", &status, &hl, &bl)
	if status == 0 {
		log.Println("unexpected input from client", status, hl, bl)
		w.WriteHeader(503)
		return invalidResponseError
	} else {
		w.WriteHeader(status)
	}
	// write back response

	headerData := make([]byte, hl)
	bodyData := make([]byte, bl)
	if n, err := io.ReadFull(c.conn, headerData); n != hl || err != nil {
		log.Println("failed to read header", n, hl, err)
		return err
	}

	if n, err := io.ReadFull(c.conn, bodyData); n != bl || err != nil {
		log.Println("failed to read body", n, bl, err)
		return err
	}
	hs := string(headerData)
	log.Println("received header", hs)
	for _, l := range strings.Split(hs, "\n") {
		var k, v string
		fmt.Sscanf(l, "%s: %s", &k, &v)
		w.Header().Add(k, v)
	}

	_, err := w.Write(bodyData)

	return err
}

var invalidResponseError = errors.New("invalid response")
var timeoutErr = errors.New("timout ")