package expose

import (
	"bufio"
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
		line, err := reader.ReadBytes('\n')
		if err != nil {
			log.Println("error reading from connection", err)
			return
		}
		log.Println("got line", string(line), " bytes ", line[:4])
		id := line[:4]
		req := decodeRequest(line[4:])
		req.URL.Scheme = "http"
		req.URL.Host = s.localEndpoint

		resp, err := s.localConn.Do(req)
		if err != nil {
			log.Println("error sending request to local", err)
			continue
		}

		encodedResp := encodeResponse(resp)
		encodedResp = append(encodedResp, '\n')
		encodedResp = append(encodedResp, id...)
		encodedResp = append(encodedResp, []byte(EndOfResponseMarker)...)
		encodedResp = append(encodedResp, '\n')
		log.Println("encoded response, sending")
		s.outboundConn.Write(append(id, encodedResp...))
		log.Println("sent response")
	}
}
