package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/magiconair/properties"
	"github.com/tiberiugal/expose"
)

func main() {

	props := properties.MustLoadFile("edgy.env", properties.UTF8)
	var cfg Config
	if err := props.Decode(&cfg); err != nil {
		log.Fatal(err)
	}

	log.Println("Starting gclient")
	srv := newServer(cfg)
	srv.run()

}

type Config struct {
	InboundServerAddr string
	LocalServerAddr   string
	DesiredNamespace  string `properties:"DesiredNamespace,default="`
}

type server struct {
	outboundConn     net.Conn
	localConn        *http.Client
	localEndpoint    string
	desiredNamespace string
}

func newServer(cfg Config) *server {
	s := &server{
		localEndpoint:    cfg.LocalServerAddr,
		desiredNamespace: cfg.DesiredNamespace,
	}
  if s.desiredNamespace == "" {
    s.desiredNamespace = expose.NoExplicitHostRequest
  }
	var err error
	s.outboundConn, err = net.Dial("tcp", cfg.InboundServerAddr)
	if err != nil {
		log.Fatal(err)
	}
	s.localConn = &http.Client{}
	return s
}

func (s *server) run() {
	s.outboundConn.Write([]byte(s.desiredNamespace + "\r\n"))

	reader := bufio.NewReader(s.outboundConn)
	ackMessage, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	log.Println("got ack", ackMessage)

	parts := strings.Split(ackMessage, " ")
	if len(parts) != 2 {
		log.Fatal("invalid ack message", ackMessage)
	}
	if parts[0] != "OK" {
		log.Fatal("invalid ack message", ackMessage)
	}
	log.Println("tunnel established to", parts[1])

	for {

		req, err := http.ReadRequest(reader)
		if err == io.EOF {
			log.Println("connection closed")
			return
		}
		if err != nil {
			log.Println("error reading from connection", err)
			continue
		}
		req.URL.Scheme = "http"
		req.URL.Host = s.localEndpoint
		req.RequestURI = ""
		resp, err := s.localConn.Do(req)
		if err != nil {
			log.Println("error sending request to local", err)
			continue
		}
		log.Println("got response", resp.Status, resp.ContentLength)
		s.outboundConn.Write([]byte("HTTP/1.1 " + resp.Status + "\r\n"))
		for k, v := range resp.Header {
			s.outboundConn.Write([]byte(k + ":" + strings.Join(v, ",") + "\r\n"))
		}
		s.outboundConn.Write([]byte("\r\n"))
		n, err := io.Copy(s.outboundConn, resp.Body)
		//s.outboundConn.Write([]byte("lorem ipsum\r\n\r\n"))
		log.Println("copied", n, err)
		fmt.Fprintf( s.outboundConn, "\r\n%s\r\n", expose.EndOfResponseMarker )
		log.Println("done")
		resp.Body.Close()

	}

}

func (s *server) handleRequest(reader *bufio.Reader, method, path string) {
	path = "http://" + s.localEndpoint + path

	http.ReadRequest(reader)

	req, err := http.NewRequest(method, path, nil)

	if err != nil {
		log.Println("error creating request", err)
		return
	}
	body := ""
	headersAreSet := false
	for {
		red, err := reader.ReadString('\n')
		log.Println("read", red, []byte(red), headersAreSet)
		if err == io.EOF {
			log.Println("connection closed")
			return
		}
		if err != nil {
			log.Println("error reading from connection", err)
			continue
		}
		if !headersAreSet {
			if red == "\r\n" {
				headersAreSet = true
				if method != "POST" && method != "PUT" {
					break
				}

			} else {
				parts := strings.Split(red, ":")
				if len(parts[1]) < 2 {
					continue
				}
				req.Header.Set(parts[0], strings.Trim(parts[1], "\r\n"))
			}
		} else {
			body += red
			if red == "\r\n" {
				break
			}
		}

	}
	if body != "" {
		req.Body = io.NopCloser(strings.NewReader(body))

	}
	log.Println("doing request", fmt.Sprintf("%+v", req.Header))
	resp, err := s.localConn.Do(req)

	if err != nil {
		log.Println("error doing request", err)
		return
	}
	defer resp.Body.Close()
	log.Println("got response", resp.Status, resp.ContentLength)
	s.outboundConn.Write([]byte("HTTP/1.1 " + resp.Status + "\r\n"))
	for k, v := range resp.Header {
		if strings.ToLower(k) == "connection" && len(v) > 0 && strings.ToLower(v[0]) == "close" {
			continue
		}
		s.outboundConn.Write([]byte(k + ":" + strings.Join(v, ",") + "\r\n"))
	}
	if resp.ContentLength != -1 {
		s.outboundConn.Write([]byte("Content-Length:" + fmt.Sprintf("%d", resp.ContentLength) + "\r\n"))
	}
	s.outboundConn.Write([]byte("\r\n"))
	n, err := io.Copy(s.outboundConn, resp.Body)
	log.Println("copied", n, err)
	s.outboundConn.Write([]byte("\r\n\r\n"))
	log.Println("done")
}
