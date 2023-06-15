package expose

import (
	"bufio"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// edgeServer proxies requests from the cloud to localEndpoint
type edgeServer struct {
	outboundConn    io.ReadWriter
	localConn       *http.Client
	localEndpoint   string
	desiredHostname string
	sessions        map[int]io.ReadWriter
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
	log.Println("edge: sent desired hostname", s.desiredHostname)
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
	log.Println("edge: got ack", ackMessage)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			log.Println("error reading from connection", err)
			return
		}
		log.Println("got line", string(line), " bytes ", line[:4])

		go func(line []byte) {
			id := line[:4]
			sessionId := decodeId(id)

			if session, exists := s.sessions[sessionId]; exists {
				session.Write(line[4:])
			}

			req := decodeRequest(line[4:])
			req.URL.Scheme = "http"
			req.URL.Host = s.localEndpoint
			if req.Header.Get("Upgrade") == "websocket" {
				go func() {

					c, _, err := websocket.DefaultDialer.Dial(s.localEndpoint+req.RequestURI, nil)
					if err != nil {
						log.Println("error dialing websocket", err)
						return
					}
					s.sessions[sessionId] = c.UnderlyingConn()
					for {
						_, msg, err := c.ReadMessage()
						if err != nil {
							log.Println("error reading websocket", err)
							return
						}
						encodedMsg := append(id, msg...)
						encodedMsg = append(encodedMsg, '\n')
						s.outboundConn.Write(encodedMsg)
					}

				}()
				return
			}
			resp, err := s.localConn.Do(req)
			if err != nil {
				log.Println("error sending request to local", err)
				return
			}

			encodedResp := append(id, encodeResponse(resp)...)
			encodedResp = append(encodedResp, '\n')
			encodedResp = append(encodedResp, id...)
			encodedResp = append(encodedResp, []byte(EndOfResponseMarker)...)
			encodedResp = append(encodedResp, '\n')
			log.Println("encoded response, sending")
			s.outboundConn.Write(encodedResp)
			log.Println("sent response")
		}(line)
	}
}
