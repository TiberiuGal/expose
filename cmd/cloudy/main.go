package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"sync"

	"github.com/tiberiugal/expose"
)

func main() {
	s := newServer()

	go func() {
		log.Println("starting incoming http server")
		http.ListenAndServe(":8080", s)

	}()

	ln, err := net.Listen("tcp", ":1044")
	if err != nil {
		log.Fatal(err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("error accepting connection", err)
			continue
		}
		err = s.Accept(conn)
		if err != nil {
			log.Println("error accepting connection", err)
			continue
		}
		log.Println("new connection accepted")
	}
}

func newServer() *server {
	s := &server{}
	s.connections = make(map[string]*http.Client)
	return s
}

// a simple http handler that receives a request and proxies it to a tcp connection
type server struct {
	connections map[string]*http.Client
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// http proxy to tcp
	// 1. get the connection
	// 2. write the request
	// 3. read the response
	// 4. write the response

	hostName := strings.Split(strings.Split(r.Host, ":")[0], ".")[0]

	conn, ok := s.connections[hostName]
	if !ok {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintln(w, "no connection found for host", hostName)
		log.Printf("no connection found for host %s, %+v \n", hostName, s.connections)
		return
	}
	pr, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)

		fmt.Fprintln(w, "error creating proxy request", err)
		return
	}
	pr.Header = r.Header
	log.Println("proxying request to", hostName)
	resp, err := conn.Do(pr)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintln(w, "error writing proxy request", err)
		return
	}
	defer resp.Body.Close()
	log.Println("got response", resp.Status)
	w.WriteHeader(resp.StatusCode)
	for k, v := range resp.Header {
		w.Header().Set(k, v[0])
		log.Println("header from response", k, v[0])
	}

	//w.Write([]byte("abc\r\n"))
	respBytes, err := ioutil.ReadAll(resp.Body)
	log.Println("read response body", len(respBytes), respBytes)
	w.Write(respBytes)
	//n, err := bufio.NewReader(resp.Body).WriteTo(w) //
	//n, err := io.Copy(w, resp.Body)
	if err != nil {
		fmt.Fprintln(w, "error writing proxy response", err)
		return
	}
	//w.Write(respBytes)
	log.Println("wrote response", resp.Status, len(respBytes))
}

func generateNewHostname() string {
	return "host" + strconv.Itoa(rand.Intn(100000))
}

func (s *server) Accept(conn net.Conn) error {
	buff := make([]byte, 1024)
	n, err := conn.Read(buff)
	if err != nil {
		log.Println("error reading from connection", err)
		return err
	}
	log.Println("read", n, "bytes from connection", buff[:n])
	host := string(buff[:n-2])
	if host == expose.NoExplicitHostRequest {
		for {
			host = generateNewHostname()
			if _, ok := s.connections[host]; !ok {
				break
			}
		}
	}

	if _, ok := s.connections[host]; ok {
		log.Println("connection already exists for host", host)
		conn.Write([]byte("ERR connection already exists\r\n"))
		return nil
	}
	conn.Write([]byte("OK " + host + "\r\n"))
	client := http.Client{
		Transport: newProxyConn(conn),
	}
	s.connections[host] = &client
	log.Println("new connection accepted for host", host)
	return nil
}

type proxyConn struct {
	conn  net.Conn
	mutex sync.Mutex
}

func newProxyConn(conn net.Conn) *proxyConn {
	p := &proxyConn{}
	p.conn = conn
	return p
}

func (p *proxyConn) RoundTrip(req *http.Request) (*http.Response, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	err := req.Write(p.conn)
	if err != nil {
		return nil, err
	}
	log.Println("wrote request", req.ContentLength, req.Method, req.URL)

	resp := &http.Response{}
	respReader := textproto.NewReader(bufio.NewReader(p.conn))
	body := ""

	firstLine, err := respReader.ReadLine()
	if err != nil {
		log.Println("error reading from connection", err)
		return nil, err
	}
	parts := strings.Split(firstLine, " ")
	if len(parts) < 3 {
		log.Println("error reading from connection", err)
		return nil, err
	}
	resp.StatusCode, _ = strconv.Atoi(parts[1])
	header, err := respReader.ReadMIMEHeader()
	if err != nil {
		log.Println("error reading from connection", err)
		return nil, err
	}
	resp.Header = make(http.Header)

	for k, v := range header {
		if len(v) > 0 {
			log.Println("header from response", k, v[0])
			resp.Header.Set(k, v[0])
		}
	}

	for {

		line, err := respReader.ReadLine()
		log.Println("read line", line)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println("error reading from connection", err)
		}
		if line == expose.EndOfResponseMarker {
			log.Println("end of response")
			break
		}
		body += line
	}

	if body != "" {
		resp.Body = io.NopCloser(strings.NewReader(body))
	}

	log.Println("read response", resp.ContentLength, len(body), resp.StatusCode)

	return resp, nil
}
