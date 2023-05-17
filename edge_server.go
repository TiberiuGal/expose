package expose

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// edgeServer proxies requests from the cloud to localEndpoint
type edgeServer struct {
	outboundConn    io.ReadWriter
	localConn       *http.Client
	localEndpoint   string
	desiredHostname string
}

// NewEdgeServer creates a new edge proxy server
func NewEdgeServer(local, namespace string, cloudConn io.ReadWriter) *edgeServer {
	s := &edgeServer{
		localEndpoint:   local,
		desiredHostname: namespace,
		outboundConn:    cloudConn,
	}
	if s.desiredHostname == "" {
		s.desiredHostname = NoExplicitHostRequest
	}
	s.localConn = &http.Client{}
	return s
}

// Run starts the proxy server
func (s *edgeServer) Run() {
	s.outboundConn.Write([]byte(s.desiredHostname + "\n"))

	reader := bufio.NewReader(s.outboundConn)
	ackMessage, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}

	parts := strings.Split(ackMessage, " ")
	if len(parts) != 2 {
		log.Fatal("invalid ack message", ackMessage)
	}
	if parts[0] != "OK" {
		log.Fatal("invalid ack message", ackMessage)
	}

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
		fmt.Fprintf(s.outboundConn, "HTTP/1.1 %s\r\n", resp.Status)
		for k, v := range resp.Header {
			fmt.Fprintf(s.outboundConn, "%s:%s\r\n", k, strings.Join(v, ","))
		}
		fmt.Fprint(s.outboundConn, "\r\n")
		_, err = io.Copy(s.outboundConn, resp.Body)

		if err != nil {
			log.Println("error copying response to cloud", err)
			continue
		}
		fmt.Fprintf(s.outboundConn, "\r\n%s\r\n", EndOfResponseMarker)
		resp.Body.Close()
	}
}
