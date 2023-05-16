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
	outboundConn     io.ReadWriter
	localConn        *http.Client
	localEndpoint    string
	desiredNamespace string
}

// NewEdgeServer creates a new edge proxy server
func NewEdgeServer(local, namespace string, cloudConn io.ReadWriter) *edgeServer {
	s := &edgeServer{
		localEndpoint:    local,
		desiredNamespace: namespace,
		outboundConn:     cloudConn,
	}
	if s.desiredNamespace == "" {
		s.desiredNamespace = NoExplicitHostRequest
	}
	s.localConn = &http.Client{}
	return s
}

// Run starts the proxy server
func (s *edgeServer) Run() {
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
		// s.outboundConn.Write([]byte("lorem ipsum\r\n\r\n"))
		log.Println("copied", n, err)
		fmt.Fprintf(s.outboundConn, "\r\n%s\r\n", EndOfResponseMarker)
		log.Println("done")
		resp.Body.Close()

	}
}
