package expose

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// edgeServer proxies requests from the cloud to localEndpoint
type edgeServer struct {
	outboundConn    io.ReadWriter
	localConn       *http.Client
	localEndpoint   string
	desiredHostname string
	sessions        map[int]*websocket.Conn
}

// NewEdgeServer creates a new edge proxy server
func NewEdgeServer(local, namespace string, cloudConn io.ReadWriter) *edgeServer {
	s := &edgeServer{
		localEndpoint:   local,
		desiredHostname: namespace,
		outboundConn:    cloudConn,
		sessions:        make(map[int]*websocket.Conn),
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
		log.Println("got line", string(line))

		go func(line []byte) {
			id := line[:4]
			sessionId := decodeId(id)

			if session, exists := s.sessions[sessionId]; exists {
				log.Println("writing to session", sessionId)
				session.WriteMessage(websocket.TextMessage, bytes.Trim(line[4:], "\n"))
				return
			}
			log.Println("no session found for id", sessionId)

			req := decodeRequest(line[4:])
			req.URL.Scheme = "http"
			req.URL.Host = s.localEndpoint
			if req.Header.Get("Upgrade") == "websocket" {
				go func(req *http.Request, id []byte) {
					log.Println("upgrading websocket", "ws://"+s.localEndpoint+req.URL.RequestURI())
					c, _, err := websocket.DefaultDialer.Dial("ws://"+s.localEndpoint+req.URL.RequestURI(), nil)
					if err != nil {
						log.Println("error dialing websocket", err)
						return
					}
					log.Println("got websocket connected")
					defer c.Close()
					c.SetReadLimit(512)
					c.SetReadDeadline(time.Now().Add(60 * time.Second))
					c.SetPongHandler(func(string) error { c.SetReadDeadline(time.Now().Add(60 * time.Second)); return nil })
					err = c.WriteMessage(websocket.TextMessage, []byte("hello"))
					if err != nil {
						log.Println("error writing to websocket", err)
						return
					}
					s.sessions[sessionId] = c
					resp := http.Response{}
					resp.StatusCode = http.StatusSwitchingProtocols
					rb := encodeResponse(&resp)
					rb = append(id, rb...)
					rb = append(rb, '\n')
					s.outboundConn.Write(rb)
					log.Println("wrote response to websocket upgrade")
					for {
						log.Println("reading from websocket")
						_, msg, err := c.ReadMessage()
						if err != nil {
							if err == io.EOF {
								log.Println("websocket closed")
								return
							}
							log.Println("error reading from websocket", err)
							return
						}
						log.Println("got message from websocket", string(msg))
						encodedMsg := append(id, msg...)
						encodedMsg = append(encodedMsg, '\n')
						n, err := s.outboundConn.Write(encodedMsg)
						if err != nil {
							log.Println("error writing to cloud", err)
							return
						}
						log.Println("wrote websocket message to cloud", n)
					}

				}(req, id)
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
